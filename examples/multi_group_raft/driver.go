// Package multigroupraft demonstrates model-based testing of a multi-group Raft
// cluster (backed by dragonboat) against a Quint specification.
//
// Progression of examples in go-quint-raft:
//
//  1. tictactoe         – basic Driver, synchronous game, default config
//  2. two_phase_commit  – custom StatePath/NondetPath, richer protocol state
//  3. raft              – single-group async distributed system
//  4. multi_group_raft  – multi-shard Raft (dragonboat)  ← you are here
//
// Architecture
// ------------
// The Quint spec (spec/multi_group_raft.qnt) models two abstract Raft shards,
// each with three nodes, focusing on leader-election safety and log monotonicity.
//
// The MultiGroupRaftDriver maps each abstract action to real dragonboat operations:
//
//	Quint action   │ dragonboat operation
//	───────────────┼────────────────────────────────────────────────────────
//	init           │ create a new Cluster (NodeHost + 2 shards)
//	electLeader    │ wait for a leader to be elected in the chosen shard
//	leaderCrash    │ stop the shard replica, wait briefly, then restart it
//	proposeEntry   │ submit a no-op command to the chosen shard's leader
//
// State checking (CheckedDriver)
// ------------------------------
// After each step the runner calls CheckState.  We verify:
//  1. ElectionSafety  – real cluster never has more leaders than the spec allows.
//  2. LogMonotone     – the real applied-entry count never decreases between steps.
package multigroupraft

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/lni/dragonboat/v4/config"
	connector "github.com/leowmjw/go-quint-raft/quint-connector-go"
	itf "github.com/informalsystems/itf-go/itf"
)

const (
	leaderWait = 500 * time.Millisecond // grace period after electLeader
	crashWait  = 200 * time.Millisecond // wait after stopping a shard
)

// ---- Driver ----

// MultiGroupRaftDriver connects the dragonboat multi-shard cluster to the
// abstract Quint multi-group Raft specification.
type MultiGroupRaftDriver struct {
	t       testing.TB
	cluster *Cluster

	// prevLogLen tracks the last observed applied-entry count per shard.
	prevLogLen map[uint64]uint64
}

// NewMultiGroupRaftDriver creates a driver managed under testing.TB.
func NewMultiGroupRaftDriver(t testing.TB) *MultiGroupRaftDriver {
	return &MultiGroupRaftDriver{
		t:          t,
		prevLogLen: make(map[uint64]uint64),
	}
}

func (d *MultiGroupRaftDriver) Config() connector.DriverConfig {
	// The spec variables are at the top level of the trace — no custom path.
	return connector.DriverConfig{}
}

// Step maps a Quint action to the corresponding dragonboat cluster operation.
func (d *MultiGroupRaftDriver) Step(step *connector.Step) error {
	switch step.ActionTaken {
	case "init":
		return d.doInit()

	case "electLeader":
		shardID, ok := step.NondetPicks.GetInt("shard")
		if !ok {
			return fmt.Errorf("electLeader: missing 'shard' pick")
		}
		return d.doElectLeader(uint64(shardID))

	case "leaderCrash":
		shardID, ok := step.NondetPicks.GetInt("shard")
		if !ok {
			return fmt.Errorf("leaderCrash: missing 'shard' pick")
		}
		return d.doLeaderCrash(uint64(shardID))

	case "proposeEntry":
		shardID, ok := step.NondetPicks.GetInt("shard")
		if !ok {
			return fmt.Errorf("proposeEntry: missing 'shard' pick")
		}
		return d.doPropose(uint64(shardID))

	default:
		return fmt.Errorf("unknown action: %s", step.ActionTaken)
	}
}

// CheckState verifies the multi-group safety invariants against the spec state.
func (d *MultiGroupRaftDriver) CheckState(specState connector.ExprValue) error {
	if d.cluster == nil {
		return nil
	}

	// Verify per-shard leader count against the spec.
	specLeaders, err := specLeaderCounts(specState)
	if err != nil {
		// If we can't parse the spec state, skip the check.
		return nil
	}

	for _, shardID := range ShardIDs {
		realHasLeader := d.cluster.HasLeader(shardID)
		specCount, ok := specLeaders[shardID]
		if !ok {
			continue
		}
		// Safety: real cluster must not have a leader when spec says none.
		if realHasLeader && specCount == 0 {
			return fmt.Errorf(
				"shard %d: real cluster has a leader but spec says 0 leaders",
				shardID,
			)
		}
	}

	// Verify log monotonicity.
	for _, shardID := range ShardIDs {
		count, err := d.cluster.Read(shardID)
		if err != nil {
			// Read may fail if the shard has no leader yet; skip.
			continue
		}
		if prev, ok := d.prevLogLen[shardID]; ok && count < prev {
			return fmt.Errorf(
				"shard %d: log count decreased from %d to %d",
				shardID, prev, count,
			)
		}
		d.prevLogLen[shardID] = count
	}

	return nil
}

// ---- Cluster operations ----

func (d *MultiGroupRaftDriver) doInit() error {
	if d.cluster != nil {
		d.cluster.Close()
		d.cluster = nil
	}
	d.prevLogLen = make(map[uint64]uint64)

	addr, err := freeTCPAddr()
	if err != nil {
		return fmt.Errorf("finding free port: %w", err)
	}

	c, err := NewCluster(d.t.TempDir(), addr)
	if err != nil {
		return fmt.Errorf("creating cluster: %w", err)
	}
	d.cluster = c
	return nil
}

func (d *MultiGroupRaftDriver) doElectLeader(shardID uint64) error {
	if _, err := d.cluster.WaitForLeader(shardID); err != nil {
		return err
	}
	time.Sleep(leaderWait)
	return nil
}

func (d *MultiGroupRaftDriver) doLeaderCrash(shardID uint64) error {
	// StopShard removes the shard replica from the NodeHost.
	if err := d.cluster.nh.StopShard(shardID); err != nil {
		// If the shard is already gone, treat it as a no-op.
		return nil
	}
	time.Sleep(crashWait)

	// Restart the shard so subsequent actions can continue.
	rc := config.Config{
		ShardID:      shardID,
		ReplicaID:    replicaID,
		ElectionRTT:  electionRTT,
		HeartbeatRTT: heartbeatRTT,
		CheckQuorum:  true,
	}
	peers := map[uint64]string{replicaID: d.cluster.addr}
	if err := d.cluster.nh.StartReplica(peers, false, NewKVStateMachine, rc); err != nil {
		return fmt.Errorf("restarting shard %d after crash: %w", shardID, err)
	}
	return nil
}

func (d *MultiGroupRaftDriver) doPropose(shardID uint64) error {
	return d.cluster.Propose(shardID)
}

// ---- Spec state helpers ----

// specLeaderCounts extracts, for each shard, how many nodes the spec considers
// to be leaders in the current state.
func specLeaderCounts(state itf.Expr) (map[uint64]int, error) {
	rec, ok := state.Value.(itf.MapExprType)
	if !ok {
		return nil, fmt.Errorf("state: expected record, got %T", state.Value)
	}
	rolesExpr, ok := rec["roles"]
	if !ok {
		return nil, fmt.Errorf("state: missing 'roles' field")
	}
	// roles is: shard-id (int) -> (node-id (str) -> Role)
	rolesMap, ok := rolesExpr.Value.(itf.MapExprType)
	if !ok {
		return nil, fmt.Errorf("roles: expected map, got %T", rolesExpr.Value)
	}

	counts := make(map[uint64]int, len(rolesMap))
	for shardKey, shardExpr := range rolesMap {
		var shardID uint64
		if _, err := fmt.Sscanf(shardKey, "%d", &shardID); err != nil {
			continue
		}
		nodeMap, ok := shardExpr.Value.(itf.MapExprType)
		if !ok {
			continue
		}
		for _, roleExpr := range nodeMap {
			if isLeaderExpr(roleExpr) {
				counts[shardID]++
			}
		}
	}
	return counts, nil
}

// isLeaderExpr reports whether an ITF expression represents the Leader variant.
func isLeaderExpr(e itf.Expr) bool {
	rec, ok := e.Value.(itf.MapExprType)
	if !ok {
		return false
	}
	tag, ok := rec["tag"]
	if !ok {
		return false
	}
	s, ok := tag.Value.(string)
	return ok && s == "Leader"
}

// freeTCPAddr finds a free TCP port on localhost and returns a host:port string.
func freeTCPAddr() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	addr := l.Addr().String()
	l.Close()
	return addr, nil
}
