package multigroupraft

import (
	"testing"

	connector "github.com/leowmjw/go-quint-raft/quint-connector-go"
)

// TestMultiGroupRaft_MBT is the primary model-based test that drives the
// dragonboat multi-shard cluster through Quint-generated traces.
//
// Each trace starts with init (cluster reset), then follows a random sequence
// of electLeader, leaderCrash, and proposeEntry steps across both shards.
//
// The test verifies:
//   - ElectionSafety: at most one leader per shard at any time.
//   - LogMonotone:    the applied-entry count never decreases.
//
// The test is skipped automatically if the quint CLI is not in PATH.
func TestMultiGroupRaft_MBT(t *testing.T) {
	connector.SkipIfNoQuint(t)

	driver := NewMultiGroupRaftDriver(t)
	t.Cleanup(func() {
		if driver.cluster != nil {
			driver.cluster.Close()
		}
	})

	connector.RunSimulation(t, driver, connector.RunConfig{
		Spec:       "spec/multi_group_raft.qnt",
		MaxSamples: 3,
		MaxSteps:   10,
	})
}

// TestMultiGroupRaft_Unit verifies the cluster and driver logic without needing
// the quint CLI.  It exercises each driver action in a realistic sequence:
//
//	init → electLeader(1) → electLeader(2) → proposeEntry(1) → proposeEntry(2)
//	     → leaderCrash(1) → electLeader(1) → proposeEntry(1)
func TestMultiGroupRaft_Unit(t *testing.T) {
	driver := NewMultiGroupRaftDriver(t)
	t.Cleanup(func() {
		if driver.cluster != nil {
			driver.cluster.Close()
		}
	})

	steps := []struct {
		name    string
		fn      func() error
	}{
		{"init", driver.doInit},
		{"electLeader(1)", func() error { return driver.doElectLeader(1) }},
		{"electLeader(2)", func() error { return driver.doElectLeader(2) }},
		{"proposeEntry(1)", func() error { return driver.doPropose(1) }},
		{"proposeEntry(2)", func() error { return driver.doPropose(2) }},
		{"leaderCrash(1)", func() error { return driver.doLeaderCrash(1) }},
		{"electLeader(1) after crash", func() error { return driver.doElectLeader(1) }},
		{"proposeEntry(1) after re-election", func() error { return driver.doPropose(1) }},
	}

	for _, s := range steps {
		if err := s.fn(); err != nil {
			t.Fatalf("step %q failed: %v", s.name, err)
		}
		t.Logf("step %q OK", s.name)
	}

	// Verify that log counts are positive after proposals.
	for _, shardID := range ShardIDs {
		count, err := driver.cluster.Read(shardID)
		if err != nil {
			t.Fatalf("reading shard %d: %v", shardID, err)
		}
		if count == 0 {
			t.Errorf("shard %d: expected at least one applied entry, got 0", shardID)
		}
		t.Logf("shard %d: %d applied entries", shardID, count)
	}
}
