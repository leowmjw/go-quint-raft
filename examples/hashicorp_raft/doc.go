// Package hashicorpraft provides a model-based test of the hashicorp/raft
// library — the production Raft implementation used in Consul, Nomad, and
// Vault — against a Quint specification.
//
// This example extends the simpler raftly-based example in examples/raft with:
//
//   - The real hashicorp/raft library (https://github.com/hashicorp/raft)
//   - A key-value FSM (SimpleFSM) to model log replication
//   - Verification that the committed FSM state matches the Quint spec
//
// Progression
// -----------
//  1. tictactoe           – basic Driver, synchronous game, default config
//  2. two_phase_commit    – custom StatePath/NondetPath, richer protocol state
//  3. raft (raftly)       – real async distributed system, election safety
//  4. hashicorp_raft      – production Raft + log replication  ← you are here
//
// Architecture
// ------------
// The Quint spec (spec/raft_hashicorp.qnt) models abstract Raft with both
// leader election and FSM replication. The HashicorpRaftDriver maps each
// abstract action to real hashicorp/raft cluster operations:
//
//	Quint action   │ hashicorp/raft operation
//	───────────────┼────────────────────────────────────────────────────────
//	init           │ create & start a fresh 3-node in-memory cluster
//	requestVote    │ disconnect leader transport → triggers re-election
//	grantVote      │ sleep (vote propagation is handled by the library)
//	becomeLeader   │ wait for the cluster to elect a new leader
//	leaderCrash    │ disconnect the current leader's transport
//	applyCommand   │ call raft.Apply(set:key=value) on the current leader
//
// State checking (CheckedDriver)
// ------------------------------
// After every step the runner calls CheckState. Two invariants are checked:
//  1. ElectionSafety: at most one real leader at any time.
//  2. FSM consistency: the leader's committed KV state matches the spec's fsm.
package hashicorpraft
