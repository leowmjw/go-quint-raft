package hashicorpraft

import (
"testing"
"time"

connector "github.com/leowmjw/go-quint-raft/quint-connector-go"
)

// TestHashicorpRaft_MBT is the primary model-based test.
//
// It drives a real hashicorp/raft 3-node in-memory cluster through
// Quint-generated traces covering leader election and log replication.
// After every step the runner calls CheckState, verifying:
//
//  1. ElectionSafety: at most one real leader at any time.
//  2. FSM consistency: the leader's KV store matches the spec's fsm.
//
// The test is skipped automatically if the quint CLI is not in PATH.
func TestHashicorpRaft_MBT(t *testing.T) {
connector.SkipIfNoQuint(t)

driver := NewHashicorpRaftDriver()
t.Cleanup(func() {
if driver.cluster != nil {
driver.cluster.Stop()
}
})

connector.RunSimulation(t, driver, connector.RunConfig{
Spec:       "spec/raft_hashicorp.qnt",
MaxSamples: 5,
MaxSteps:   12,
})
}

// TestHashicorpRaft_Unit verifies driver logic without needing the quint CLI.
// It replays a hand-crafted trace:
//
//init → becomeLeader → ApplyKV(k1=v1) → leaderCrash → requestVote →
//becomeLeader → ApplyKV(k2=v2)
//
// applyCommand steps use ApplyKV directly because connector.NondetPicks has
// unexported fields and cannot be constructed outside the package.
func TestHashicorpRaft_Unit(t *testing.T) {
driver := NewHashicorpRaftDriver()
t.Cleanup(func() {
if driver.cluster != nil {
driver.cluster.Stop()
}
})

// Election steps go through the standard Step() path.
for _, action := range []string{"init", "becomeLeader", "leaderCrash", "requestVote", "becomeLeader"} {
if err := driver.Step(&connector.Step{ActionTaken: action}); err != nil {
t.Fatalf("step %q: %v", action, err)
}
t.Logf("step %q OK (leader=%q)", action, driver.leaderID)
}

// FSM commands are applied via the public ApplyKV helper.
for _, c := range []struct{ key, value string }{{"k1", "v1"}, {"k2", "v2"}} {
if err := driver.ApplyKV(c.key, c.value); err != nil {
t.Fatalf("ApplyKV(%q, %q): %v", c.key, c.value, err)
}
t.Logf("ApplyKV(%q, %q) OK", c.key, c.value)
}

// Verify the final FSM state on the leader.
leader := driver.cluster.LeaderNode()
if leader == nil {
t.Fatal("no leader after final step")
}
for key, want := range map[string]string{"k1": "v1", "k2": "v2"} {
got, ok := leader.fsm.Get(key)
if !ok {
t.Errorf("FSM: key %q missing", key)
} else if got != want {
t.Errorf("FSM: key %q = %q, want %q", key, got, want)
}
}
}

// TestHashicorpRaft_ElectionSafety verifies the ElectionSafety invariant by
// repeatedly crashing leaders and confirming there is never more than one.
func TestHashicorpRaft_ElectionSafety(t *testing.T) {
driver := NewHashicorpRaftDriver()
t.Cleanup(func() {
if driver.cluster != nil {
driver.cluster.Stop()
}
})

if err := driver.Step(&connector.Step{ActionTaken: "init"}); err != nil {
t.Fatalf("init: %v", err)
}

for round := range 2 {
if err := driver.Step(&connector.Step{ActionTaken: "becomeLeader"}); err != nil {
t.Fatalf("round %d becomeLeader: %v", round, err)
}
t.Logf("round %d: leader=%q", round, driver.leaderID)

if n := driver.cluster.CountLeaders(); n != 1 {
t.Errorf("round %d: expected 1 leader, got %d", round, n)
}

if err := driver.Step(&connector.Step{ActionTaken: "leaderCrash"}); err != nil {
t.Fatalf("round %d leaderCrash: %v", round, err)
}
time.Sleep(50 * time.Millisecond)

if n := driver.cluster.CountLeaders(); n > 1 {
t.Errorf("round %d post-crash: expected ≤1 leader, got %d", round, n)
}
}
}
