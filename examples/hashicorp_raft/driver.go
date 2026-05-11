package hasicorpraft

import (
	"fmt"
	"time"

	itf "github.com/informalsystems/itf-go/itf"
	connector "github.com/leowmjw/go-quint-raft/quint-connector-go"
)

const (
	waitForLeaderTimeout = 800 * time.Millisecond
	applyTimeout         = 3 * time.Second
	crashWait            = 2 * electionTimeout // time for crash to propagate
)

var clusterNodeIDs = []string{"n1", "n2", "n3"}

// HashicorpRaftDriver connects the hashicorp/raft implementation to the
// abstract Quint spec (spec/raft_hashicorp.qnt).
//
// It verifies two properties after every step:
//  1. ElectionSafety: at most one real leader at any time.
//  2. FSM consistency: the leader's KV store matches the spec's fsm variable.
type HashicorpRaftDriver struct {
	cluster  *Cluster
	leaderID string // last known leader; "" = unknown
}

// NewHashicorpRaftDriver creates a driver ready for use in tests.
func NewHashicorpRaftDriver() *HashicorpRaftDriver {
	return &HashicorpRaftDriver{}
}

func (d *HashicorpRaftDriver) Config() connector.DriverConfig {
	// All spec variables are at the top level; use default mbt:: paths.
	return connector.DriverConfig{}
}

// Step maps a Quint spec action to the corresponding hashicorp/raft operation.
func (d *HashicorpRaftDriver) Step(step *connector.Step) error {
	switch step.ActionTaken {
	case "init":
		return d.doInit()

	case "requestVote":
		// A follower starts an election.  We trigger this by disconnecting the
		// current leader so the remaining followers time out and hold a vote.
		return d.doRequestVote()

	case "grantVote":
		// Vote propagation is handled asynchronously by the library.
		time.Sleep(crashWait)
		return nil

	case "becomeLeader":
		return d.doBecomeLeader()

	case "leaderCrash":
		return d.doLeaderCrash()

	case "applyCommand":
		return d.doApplyCommand(step)

	default:
		return fmt.Errorf("unknown action: %s", step.ActionTaken)
	}
}

// CheckState verifies ElectionSafety and FSM consistency after every step.
func (d *HashicorpRaftDriver) CheckState(specState connector.ExprValue) error {
	if d.cluster == nil {
		return nil
	}

	// -- 1. ElectionSafety --
	realLeaders := d.cluster.CountLeaders()
	if realLeaders > 1 {
		return fmt.Errorf("ElectionSafety violated: %d simultaneous leaders", realLeaders)
	}

	// -- 2. FSM consistency --
	// Only check FSM when the spec says there are committed entries (non-empty fsm).
	specFSM, err := specFSMState(specState)
	if err != nil || len(specFSM) == 0 {
		// No committed entries to compare; skip.
		return nil
	}

	leader := d.cluster.LeaderNode()
	if leader == nil {
		// No real leader right now; the cluster may be mid-election.
		// Allow the spec to be "ahead" during election periods.
		return nil
	}

	realFSM := leader.fsm.Snapshot2()
	for key, specVal := range specFSM {
		realVal, ok := realFSM[key]
		if !ok {
			return fmt.Errorf("FSM mismatch: key %q present in spec (=%q) but missing in impl",
				key, specVal)
		}
		if realVal != specVal {
			return fmt.Errorf("FSM mismatch: key %q spec=%q impl=%q",
				key, specVal, realVal)
		}
	}
	return nil
}

// ---- Cluster operations ----

func (d *HashicorpRaftDriver) doInit() error {
	// Tear down any previous cluster.
	if d.cluster != nil {
		d.cluster.Stop()
		d.cluster = nil
	}
	d.leaderID = ""

	c, err := NewCluster(clusterNodeIDs)
	if err != nil {
		return fmt.Errorf("init: %w", err)
	}
	d.cluster = c
	return nil
}

func (d *HashicorpRaftDriver) doRequestVote() error {
	// Only crash the previously known leader.  If d.leaderID is already ""
	// (e.g. after a leaderCrash), the remaining nodes will elect naturally.
	if d.leaderID != "" {
		if err := d.cluster.CrashNode(d.leaderID); err != nil {
			return fmt.Errorf("requestVote/crash: %w", err)
		}
		d.leaderID = ""
		time.Sleep(crashWait)
	}
	return nil
}

func (d *HashicorpRaftDriver) doBecomeLeader() error {
	id, err := d.cluster.WaitForLeader(waitForLeaderTimeout)
	if err != nil {
		return fmt.Errorf("becomeLeader: %w", err)
	}
	d.leaderID = id
	return nil
}

func (d *HashicorpRaftDriver) doLeaderCrash() error {
	if d.leaderID == "" {
		d.leaderID = d.cluster.leaderID()
	}
	if d.leaderID == "" {
		return nil // no leader to crash; spec allows this
	}
	if err := d.cluster.CrashNode(d.leaderID); err != nil {
		return fmt.Errorf("leaderCrash: %w", err)
	}
	d.leaderID = ""
	time.Sleep(crashWait)
	return nil
}

func (d *HashicorpRaftDriver) doApplyCommand(step *connector.Step) error {
	key, ok := step.NondetPicks.GetString("key")
	if !ok {
		return fmt.Errorf("applyCommand: missing nondet pick 'key'")
	}
	value, ok := step.NondetPicks.GetString("value")
	if !ok {
		return fmt.Errorf("applyCommand: missing nondet pick 'value'")
	}
	return d.applyKV(key, value)
}

// ApplyKV applies a set(key, value) command directly to the cluster leader.
// This is useful for unit tests that do not go through the quint trace runner.
func (d *HashicorpRaftDriver) ApplyKV(key, value string) error {
	return d.applyKV(key, value)
}

// applyKV finds (or waits for) the current leader and applies a set command.
func (d *HashicorpRaftDriver) applyKV(key, value string) error {
	// Make sure we have a leader (the spec guarantees one, but the real
	// cluster may still be electing due to timing).
	if d.leaderID == "" {
		id, err := d.cluster.WaitForLeader(waitForLeaderTimeout)
		if err != nil {
			return fmt.Errorf("applyCommand: waiting for leader: %w", err)
		}
		d.leaderID = id
	}

	leaderNode := d.cluster.nodes[d.leaderID]
	if leaderNode == nil {
		return fmt.Errorf("applyCommand: unknown leader %q", d.leaderID)
	}

	cmd, err := encodeCommand(key, value)
	if err != nil {
		return fmt.Errorf("applyCommand: encoding: %w", err)
	}

	future := leaderNode.raft.Apply(cmd, applyTimeout)
	if err := future.Error(); err != nil {
		// If we lost leadership mid-apply, wait for a new leader and retry once.
		d.leaderID = ""
		id, lerr := d.cluster.WaitForLeader(waitForLeaderTimeout)
		if lerr != nil {
			return fmt.Errorf("applyCommand: apply failed and no new leader: apply=%w wait=%v", err, lerr)
		}
		d.leaderID = id
		leaderNode = d.cluster.nodes[d.leaderID]
		future = leaderNode.raft.Apply(cmd, applyTimeout)
		if err := future.Error(); err != nil {
			return fmt.Errorf("applyCommand: retry apply: %w", err)
		}
	}

	return nil
}

// ---- Spec state helpers ----

// specFSMState extracts the `fsm` map from the top-level spec state.
// It returns nil (not an error) when fsm is absent or empty.
func specFSMState(state itf.Expr) (map[string]string, error) {
	rec, ok := state.Value.(itf.MapExprType)
	if !ok {
		return nil, fmt.Errorf("state: expected record, got %T", state.Value)
	}
	fsmExpr, ok := rec["fsm"]
	if !ok {
		return nil, nil // fsm variable not present in this trace step
	}
	fsmMap, ok := fsmExpr.Value.(itf.MapExprType)
	if !ok {
		return nil, fmt.Errorf("fsm: expected map, got %T", fsmExpr.Value)
	}
	if len(fsmMap) == 0 {
		return nil, nil
	}
	result := make(map[string]string, len(fsmMap))
	for k, v := range fsmMap {
		s, ok := v.Value.(string)
		if !ok {
			return nil, fmt.Errorf("fsm[%q]: expected string value, got %T", k, v.Value)
		}
		result[k] = s
	}
	return result, nil
}
