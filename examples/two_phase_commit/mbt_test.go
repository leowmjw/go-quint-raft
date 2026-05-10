package twophasecommit

import (
	"fmt"
	"testing"

	connector "github.com/leowmjw/go-quint-raft/quint-connector-go"
	itf "github.com/informalsystems/itf-go/itf"
)

// nodeIDs is the fixed set of participants plus coordinator, matching the Quint spec.
var nodeIDs = []string{"c", "p1", "p2", "p3"}

const coordinatorID = "c"
const numParticipants = 3

// ---- Driver ----

// TwoPhaseCommitDriver connects the Go 2PC implementation to the Quint specification.
//
// This is a Go port of the Rust TwoPhaseCommitDriver in mbt.rs.
//
// Key points that distinguish this example from tictactoe:
//   - Config uses custom StatePath and NondetPath to navigate nested spec state
//   - The nondet picks come from a sum-type (actionTaken field) not mbt:: vars
//   - State checking compares each node's Stage against the spec's LocalState
type TwoPhaseCommitDriver struct {
	nodes        map[string]Node
	messagesSent map[string]map[Message]struct{}
}

func (d *TwoPhaseCommitDriver) Config() connector.DriverConfig {
	return connector.DriverConfig{
		// The 2PC spec uses the Choreo framework; the system state lives at:
		//   two_phase_commit::choreo::s → system
		StatePath: []string{"two_phase_commit::choreo::s", "system"},

		// The action taken is recorded as a sum type in:
		//   two_phase_commit::choreo::s → extensions → actionTaken
		NondetPath: []string{"two_phase_commit::choreo::s", "extensions", "actionTaken"},
	}
}

// Step dispatches a Quint trace step to the corresponding Go method.
// Action names match the sum-type tags in the Quint spec's ActionTaken type.
func (d *TwoPhaseCommitDriver) Step(step *connector.Step) error {
	node, _ := step.NondetPicks.GetString("node")

	switch step.ActionTaken {
	case "Init":
		d.init()

	case "SpontaneouslyPrepares":
		d.spontaneouslyPrepares(node)

	case "SpontaneouslyAborts":
		d.spontaneouslyAborts(node)

	case "AbortsAsInstructed":
		d.abortsAsInstructed(node)

	case "CommitsAsInstructed":
		d.commitsAsInstructed(node)

	case "DecidesOnCommit":
		d.decidesOnCommit(node)

	case "DecidesOnAbort":
		d.decidesOnAbort(node)

	default:
		return fmt.Errorf("unknown action: %s", step.ActionTaken)
	}
	return nil
}

// CheckState validates each node's Stage against the specification state.
// The specState value has been narrowed to the StatePath, so it is the
// `system` map: node-id → LocalState with a `stage` field.
func (d *TwoPhaseCommitDriver) CheckState(specState connector.ExprValue) error {
	systemMap, ok := specState.Value.(itf.MapExprType)
	if !ok {
		return fmt.Errorf("system state: expected map, got %T", specState.Value)
	}

	for id, implNode := range d.nodes {
		nodeExpr, ok := systemMap[id]
		if !ok {
			return fmt.Errorf("node %q missing from spec state", id)
		}
		specStage, err := stageFromExpr(nodeExpr)
		if err != nil {
			return fmt.Errorf("node %q stage: %w", id, err)
		}
		if implNode.Stage() != specStage {
			return fmt.Errorf("node %q: impl stage=%v spec stage=%v",
				id, implNode.Stage(), specStage)
		}
	}
	return nil
}

// stageFromExpr extracts a Stage value from a LocalState ITF expression.
// The LocalState is a record like {process_id: "p1", role: {...}, stage: {tag:"Prepared"}}.
func stageFromExpr(localStateExpr itf.Expr) (Stage, error) {
	rec, ok := localStateExpr.Value.(itf.MapExprType)
	if !ok {
		return 0, fmt.Errorf("expected record, got %T", localStateExpr.Value)
	}
	stageExpr, ok := rec["stage"]
	if !ok {
		return 0, fmt.Errorf("missing 'stage' field")
	}
	// Stage is a sum type in Quint: {tag: "Working"|"Prepared"|"Committed"|"Aborted"}
	stageRec, ok := stageExpr.Value.(itf.MapExprType)
	if !ok {
		return 0, fmt.Errorf("stage: expected record, got %T", stageExpr.Value)
	}
	tagExpr, ok := stageRec["tag"]
	if !ok {
		return 0, fmt.Errorf("stage: missing tag")
	}
	tag, ok := tagExpr.Value.(string)
	if !ok {
		return 0, fmt.Errorf("stage: tag not string")
	}
	switch tag {
	case "Working":
		return Working, nil
	case "Prepared":
		return Prepared, nil
	case "Committed":
		return Committed, nil
	case "Aborted":
		return Aborted, nil
	default:
		return 0, fmt.Errorf("stage: unknown tag %q", tag)
	}
}

// ---- Protocol operations ----

func (d *TwoPhaseCommitDriver) init() {
	d.nodes = make(map[string]Node, len(nodeIDs))
	d.messagesSent = make(map[string]map[Message]struct{}, len(nodeIDs))

	for _, id := range nodeIDs {
		sent := make(map[Message]struct{})
		if id == coordinatorID {
			coord := NewCoordinator(numParticipants)
			sent[coord.Start()] = struct{}{} // broadcast MsgPrepare at start
			d.nodes[id] = coord
		} else {
			d.nodes[id] = &Participant{}
		}
		d.messagesSent[id] = sent
	}
}

func (d *TwoPhaseCommitDriver) spontaneouslyPrepares(nodeID string) {
	prepareMsg := d.prepareMsg()
	reply := d.nodes[nodeID].Receive(prepareMsg)
	d.recordReply(nodeID, reply)
}

func (d *TwoPhaseCommitDriver) spontaneouslyAborts(nodeID string) {
	reply := d.nodes[nodeID].Timeout()
	d.recordReply(nodeID, reply)
}

func (d *TwoPhaseCommitDriver) abortsAsInstructed(nodeID string) {
	abortMsg := d.abortMsg()
	reply := d.nodes[nodeID].Receive(abortMsg)
	d.recordReply(nodeID, reply)
}

func (d *TwoPhaseCommitDriver) commitsAsInstructed(nodeID string) {
	commitMsg := d.commitMsg()
	reply := d.nodes[nodeID].Receive(commitMsg)
	d.recordReply(nodeID, reply)
}

func (d *TwoPhaseCommitDriver) decidesOnCommit(nodeID string) {
	for _, msg := range d.preparedMsgs() {
		reply := d.nodes[nodeID].Receive(msg)
		d.recordReply(nodeID, reply)
	}
}

func (d *TwoPhaseCommitDriver) decidesOnAbort(nodeID string) {
	reply := d.nodes[nodeID].Timeout()
	d.recordReply(nodeID, reply)
}

func (d *TwoPhaseCommitDriver) recordReply(nodeID string, reply *Message) {
	if reply != nil {
		d.messagesSent[nodeID][*reply] = struct{}{}
	}
}

func (d *TwoPhaseCommitDriver) prepareMsg() Message {
	for msg := range d.messagesSent[coordinatorID] {
		if msg == MsgPrepare {
			return MsgPrepare
		}
	}
	panic("coordinator has not sent MsgPrepare")
}

func (d *TwoPhaseCommitDriver) abortMsg() Message {
	for msg := range d.messagesSent[coordinatorID] {
		if msg == MsgAbort {
			return MsgAbort
		}
	}
	panic("coordinator has not sent MsgAbort")
}

func (d *TwoPhaseCommitDriver) commitMsg() Message {
	for msg := range d.messagesSent[coordinatorID] {
		if msg == MsgCommit {
			return MsgCommit
		}
	}
	panic("coordinator has not sent MsgCommit")
}

func (d *TwoPhaseCommitDriver) preparedMsgs() []Message {
	var msgs []Message
	for _, sent := range d.messagesSent {
		for msg := range sent {
			if msg == MsgPrepared {
				msgs = append(msgs, msg)
			}
		}
	}
	return msgs
}

// ---- Tests ----

// TestTwoPhaseCommitSimulation runs the 2PC simulation using quint-connector-go.
// This is a Go port of the #[quint_run] test in mbt.rs.
func TestTwoPhaseCommitSimulation(t *testing.T) {
	connector.SkipIfNoQuint(t)

	driver := &TwoPhaseCommitDriver{}
	connector.RunSimulation(t, driver, connector.RunConfig{
		Spec:       "spec/two_phase_commit.qnt",
		MaxSamples: 1,
	})
}

// TestTwoPhaseCommitHappyPath runs the commitTest scenario from the Quint spec.
// This is a Go port of the #[quint_test] test in mbt.rs.
func TestTwoPhaseCommitHappyPath(t *testing.T) {
	connector.SkipIfNoQuint(t)

	driver := &TwoPhaseCommitDriver{}
	connector.RunTest(t, driver, connector.TestConfig{
		Spec:       "spec/two_phase_commit.qnt",
		Test:       "commitTest",
		MaxSamples: 1,
	})
}
