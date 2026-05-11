package simpleraft

import (
	"fmt"

	itf "github.com/informalsystems/itf-go/itf"
	connector "github.com/leowmjw/go-quint-raft/quint-connector-go"
)

type role string

const (
	follower  role = "Follower"
	candidate role = "Candidate"
	leader    role = "Leader"
	quorum         = 2
)

var nodeIDs = []string{"node0", "node1", "node2"}

type nodeState struct {
	Role     role
	Term     int64
	VotedFor string
	Alive    bool
}

type SimpleRaftDriver struct {
	nodes map[string]nodeState
	votes map[string]map[string]struct{}
}

func (d *SimpleRaftDriver) Config() connector.DriverConfig {
	return connector.DriverConfig{}
}

func (d *SimpleRaftDriver) Step(step *connector.Step) error {
	switch step.ActionTaken {
	case "init":
		d.init()
	case "triggerElection":
		node := d.pickNode(step, "node", nodeIDs[0])
		d.triggerElection(node)
	case "grantVote":
		voter := d.pickNode(step, "voter", nodeIDs[1])
		candidateID := d.pickNode(step, "candidate", nodeIDs[0])
		d.grantVote(voter, candidateID)
	case "becomeLeader":
		node := d.pickNode(step, "node", nodeIDs[0])
		d.becomeLeader(node)
	case "crashLeader":
		node := d.pickNode(step, "node", nodeIDs[0])
		d.crashLeader(node)
	case "restartNode":
		node := d.pickNode(step, "node", nodeIDs[0])
		d.restartNode(node)
	default:
		return fmt.Errorf("unknown action: %s", step.ActionTaken)
	}
	return nil
}

func (d *SimpleRaftDriver) CheckState(specState connector.ExprValue) error {
	rec, ok := specState.Value.(itf.MapExprType)
	if !ok {
		return fmt.Errorf("state: expected record, got %T", specState.Value)
	}

	roles, err := mapField(rec, "roles")
	if err != nil {
		return err
	}
	terms, err := mapField(rec, "terms")
	if err != nil {
		return err
	}
	votedFor, err := mapField(rec, "votedFor")
	if err != nil {
		return err
	}
	alive, err := mapField(rec, "alive")
	if err != nil {
		return err
	}
	votes, err := mapField(rec, "votes")
	if err != nil {
		return err
	}

	for _, id := range nodeIDs {
		n, ok := d.nodes[id]
		if !ok {
			return fmt.Errorf("impl missing node %q", id)
		}

		specRole, err := roleFromExpr(roles[id])
		if err != nil {
			return fmt.Errorf("roles[%s]: %w", id, err)
		}
		if n.Role != specRole {
			return fmt.Errorf("roles[%s]: impl=%s spec=%s", id, n.Role, specRole)
		}

		specTerm, err := intFromExpr(terms[id])
		if err != nil {
			return fmt.Errorf("terms[%s]: %w", id, err)
		}
		if n.Term != specTerm {
			return fmt.Errorf("terms[%s]: impl=%d spec=%d", id, n.Term, specTerm)
		}

		specVotedFor, err := stringFromExpr(votedFor[id])
		if err != nil {
			return fmt.Errorf("votedFor[%s]: %w", id, err)
		}
		if n.VotedFor != specVotedFor {
			return fmt.Errorf("votedFor[%s]: impl=%q spec=%q", id, n.VotedFor, specVotedFor)
		}

		specAlive, err := boolFromExpr(alive[id])
		if err != nil {
			return fmt.Errorf("alive[%s]: %w", id, err)
		}
		if n.Alive != specAlive {
			return fmt.Errorf("alive[%s]: impl=%t spec=%t", id, n.Alive, specAlive)
		}

		specVotes, err := setFromExpr(votes[id])
		if err != nil {
			return fmt.Errorf("votes[%s]: %w", id, err)
		}
		implVotes := d.votes[id]
		if len(implVotes) != len(specVotes) {
			return fmt.Errorf("votes[%s]: impl=%d entries spec=%d entries", id, len(implVotes), len(specVotes))
		}
		for v := range implVotes {
			if _, ok := specVotes[v]; !ok {
				return fmt.Errorf("votes[%s]: impl has %q but spec does not", id, v)
			}
		}
	}

	return nil
}

func (d *SimpleRaftDriver) pickNode(step *connector.Step, name, fallback string) string {
	if node, ok := step.NondetPicks.GetString(name); ok {
		return node
	}
	return fallback
}

func (d *SimpleRaftDriver) init() {
	d.nodes = make(map[string]nodeState, len(nodeIDs))
	d.votes = make(map[string]map[string]struct{}, len(nodeIDs))
	for _, id := range nodeIDs {
		d.nodes[id] = nodeState{Role: follower, Term: 0, VotedFor: "", Alive: true}
		d.votes[id] = make(map[string]struct{})
	}
}

func (d *SimpleRaftDriver) triggerElection(node string) {
	n := d.nodes[node]
	if !n.Alive || n.Role == leader {
		return
	}
	n.Role = candidate
	n.Term++
	n.VotedFor = node
	d.nodes[node] = n
	d.votes[node] = map[string]struct{}{node: {}}
}

func (d *SimpleRaftDriver) grantVote(voter, candidateID string) {
	v := d.nodes[voter]
	c := d.nodes[candidateID]
	if !v.Alive || !c.Alive || voter == candidateID {
		return
	}
	if c.Role != candidate || v.Role != follower || v.VotedFor != "" || c.Term < v.Term {
		return
	}
	v.Term = c.Term
	v.VotedFor = candidateID
	d.nodes[voter] = v
	if d.votes[candidateID] == nil {
		d.votes[candidateID] = make(map[string]struct{})
	}
	d.votes[candidateID][voter] = struct{}{}
}

func (d *SimpleRaftDriver) becomeLeader(node string) {
	n := d.nodes[node]
	if !n.Alive || n.Role != candidate {
		return
	}
	if len(d.votes[node]) < quorum {
		return
	}
	n.Role = leader
	d.nodes[node] = n
}

func (d *SimpleRaftDriver) crashLeader(node string) {
	n := d.nodes[node]
	if !n.Alive || n.Role != leader {
		return
	}
	n.Alive = false
	n.Role = follower
	n.VotedFor = ""
	d.nodes[node] = n
	d.votes[node] = make(map[string]struct{})
}

func (d *SimpleRaftDriver) restartNode(node string) {
	n := d.nodes[node]
	if n.Alive {
		return
	}
	n.Alive = true
	n.Role = follower
	n.VotedFor = ""
	d.nodes[node] = n
	d.votes[node] = make(map[string]struct{})
}

func mapField(rec itf.MapExprType, key string) (itf.MapExprType, error) {
	expr, ok := rec[key]
	if !ok {
		return nil, fmt.Errorf("state: missing %q", key)
	}
	m, ok := expr.Value.(itf.MapExprType)
	if !ok {
		return nil, fmt.Errorf("state[%s]: expected map, got %T", key, expr.Value)
	}
	return m, nil
}

func roleFromExpr(expr itf.Expr) (role, error) {
	rec, ok := expr.Value.(itf.MapExprType)
	if !ok {
		return "", fmt.Errorf("expected role record, got %T", expr.Value)
	}
	tagExpr, ok := rec["tag"]
	if !ok {
		return "", fmt.Errorf("missing role tag")
	}
	tag, ok := tagExpr.Value.(string)
	if !ok {
		return "", fmt.Errorf("role tag is not string")
	}
	switch tag {
	case string(follower), string(candidate), string(leader):
		return role(tag), nil
	default:
		return "", fmt.Errorf("unknown role %q", tag)
	}
}

func intFromExpr(expr itf.Expr) (int64, error) {
	switch v := expr.Value.(type) {
	case int64:
		return v, nil
	case float64:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("expected int, got %T", expr.Value)
	}
}

func stringFromExpr(expr itf.Expr) (string, error) {
	s, ok := expr.Value.(string)
	if !ok {
		return "", fmt.Errorf("expected string, got %T", expr.Value)
	}
	return s, nil
}

func boolFromExpr(expr itf.Expr) (bool, error) {
	b, ok := expr.Value.(bool)
	if !ok {
		return false, fmt.Errorf("expected bool, got %T", expr.Value)
	}
	return b, nil
}

func setFromExpr(expr itf.Expr) (map[string]struct{}, error) {
	list, ok := expr.Value.(itf.ListExprType)
	if !ok {
		return nil, fmt.Errorf("expected set/list, got %T", expr.Value)
	}
	out := make(map[string]struct{}, len(list))
	for _, item := range list {
		s, ok := item.Value.(string)
		if !ok {
			return nil, fmt.Errorf("set element is %T, expected string", item.Value)
		}
		out[s] = struct{}{}
	}
	return out, nil
}
