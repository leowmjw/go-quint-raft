// Package twophasecommit implements a two-phase commit protocol for model-based testing.
//
// This is a Go port of the two_phase_commit example from quint-connect (Rust).
// It shows an intermediate use of quint-connector-go with:
//   - A custom NondetPath pointing to a sum-type actionTaken field
//   - A custom StatePath navigating nested spec state
//   - Full state checking comparing implementation stages to the spec
//
// Progression from simple to complex:
//   1. tictactoe  – basic Driver, default config, state checking via CheckedDriver
//   2. two_phase_commit – custom StatePath + NondetPath, more realistic protocol  ← you are here
//   3. raft         – real distributed system, asynchronous operations
package twophasecommit

// Stage represents the lifecycle state of a 2PC participant or coordinator.
type Stage int

const (
	Working   Stage = iota // initial state; ready to participate
	Prepared               // participant has voted to commit
	Committed              // final committed state
	Aborted                // final aborted state
)

func (s Stage) String() string {
	return [...]string{"Working", "Prepared", "Committed", "Aborted"}[s]
}

// Message is a protocol message exchanged between nodes.
type Message int

const (
	MsgPrepare   Message = iota // coordinator → participants: "please prepare"
	MsgPrepared                 // participant → coordinator: "I am prepared"
	MsgCommit                   // coordinator → participants: "please commit"
	MsgAbort                    // coordinator → participants: "please abort"
)

// Node is the common interface for both coordinator and participant.
type Node interface {
	Stage() Stage
	Timeout() *Message // may return nil if nothing to do
	Receive(msg Message) *Message
}

// ---- Coordinator ----

// Coordinator manages the two-phase commit protocol.
type Coordinator struct {
	stage    Stage
	quorum   int
	prepared int
}

// NewCoordinator creates a coordinator that requires `quorum` prepared votes.
func NewCoordinator(quorum int) *Coordinator {
	return &Coordinator{stage: Working, quorum: quorum}
}

// Start returns the first message the coordinator broadcasts (MsgPrepare).
func (c *Coordinator) Start() Message { return MsgPrepare }

func (c *Coordinator) Stage() Stage { return c.stage }

func (c *Coordinator) Timeout() *Message {
	if c.stage == Working {
		c.stage = Aborted
		m := MsgAbort
		return &m
	}
	return nil
}

func (c *Coordinator) Receive(msg Message) *Message {
	if c.stage == Working && msg == MsgPrepared {
		c.prepared++
		if c.prepared == c.quorum {
			c.stage = Committed
			m := MsgCommit
			return &m
		}
	}
	return nil
}

// ---- Participant ----

// Participant is a 2PC participant node.
type Participant struct {
	stage Stage
}

func (p *Participant) Stage() Stage { return p.stage }

func (p *Participant) Timeout() *Message {
	if p.stage == Working {
		p.stage = Aborted
	}
	return nil
}

func (p *Participant) Receive(msg Message) *Message {
	switch {
	case p.stage == Working && msg == MsgPrepare:
		p.stage = Prepared
		m := MsgPrepared
		return &m
	case msg == MsgAbort:
		p.stage = Aborted
	case msg == MsgCommit:
		p.stage = Committed
	}
	return nil
}
