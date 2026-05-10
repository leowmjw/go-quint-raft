package raft

import (
	"testing"

	connector "github.com/leowmjw/go-quint-raft/quint-connector-go"
)

// TestRaftLeaderElection_MBT is the primary model-based test that drives the
// raftly cluster through Quint-generated leader-election traces.
//
// Each trace starts with init (cluster reset), then follows a random
// sequence of requestVote → grantVote → becomeLeader → leaderCrash steps.
//
// The test verifies Raft's ElectionSafety invariant after every step:
// at most one leader may exist at any time.
//
// The test is skipped automatically if the quint CLI is not in PATH.
func TestRaftLeaderElection_MBT(t *testing.T) {
	connector.SkipIfNoQuint(t)

	driver := NewRaftDriver(t)
	t.Cleanup(func() {
		if driver.cluster != nil {
			driver.cluster.Stop()
			driver.cluster.Cleanup()
		}
	})

	connector.RunSimulation(t, driver, connector.RunConfig{
		Spec:       "spec/raft_election.qnt",
		MaxSamples: 5,
		MaxSteps:   10,
	})
}

// TestRaftLeaderElection_Unit verifies driver logic without needing quint CLI.
// It replays a hand-crafted trace representing a simple election cycle.
func TestRaftLeaderElection_Unit(t *testing.T) {
	driver := NewRaftDriver(t)
	t.Cleanup(func() {
		if driver.cluster != nil {
			driver.cluster.Stop()
			driver.cluster.Cleanup()
		}
	})

	// init → becomeLeader → leaderCrash → requestVote → becomeLeader
	steps := []struct {
		action string
	}{
		{"init"},
		{"becomeLeader"},
		{"leaderCrash"},
		{"requestVote"},
		{"becomeLeader"},
	}

	for _, s := range steps {
		step := &connector.Step{ActionTaken: s.action}
		if err := driver.Step(step); err != nil {
			t.Fatalf("step %q failed: %v", s.action, err)
		}
		t.Logf("step %q OK (leader=%q)", s.action, driver.leaderID)
	}
}
