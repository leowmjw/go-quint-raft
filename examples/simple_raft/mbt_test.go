package simpleraft

import (
	"testing"

	connector "github.com/leowmjw/go-quint-raft/quint-connector-go"
)

func TestSimpleRaft_MBT(t *testing.T) {
	connector.SkipIfNoQuint(t)

	driver := &SimpleRaftDriver{}
	connector.RunSimulation(t, driver, connector.RunConfig{
		Spec:       "spec/simple_raft.qnt",
		MaxSamples: 5,
		MaxSteps:   20,
	})
}

func TestSimpleRaft_Unit(t *testing.T) {
	driver := &SimpleRaftDriver{}

	steps := []connector.Step{
		{ActionTaken: "init"},
		{ActionTaken: "triggerElection"},
		{ActionTaken: "grantVote"},
		{ActionTaken: "becomeLeader"},
		{ActionTaken: "crashLeader"},
		{ActionTaken: "restartNode"},
	}

	for _, step := range steps {
		if err := driver.Step(&step); err != nil {
			t.Fatalf("step %q failed: %v", step.ActionTaken, err)
		}
	}
}
