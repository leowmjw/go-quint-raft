package hasicorpraft

import (
	"fmt"
	"time"

	hraft "github.com/hashicorp/raft"
)

const (
	// electionTimeout is used for both HeartbeatTimeout and ElectionTimeout in
	// the raft config.  Shorter values make tests faster.
	electionTimeout = 80 * time.Millisecond
)

// node bundles everything belonging to a single raft peer.
type node struct {
	raft      *hraft.Raft
	transport *hraft.InmemTransport
	fsm       *SimpleFSM
}

// Cluster manages a 3-node in-memory hashicorp/raft cluster.
type Cluster struct {
	nodes   map[string]*node
	nodeIDs []string
	crashed map[string]bool
}

// NewCluster creates a fresh 3-node hashicorp/raft cluster and bootstraps it.
// nodeIDs must contain exactly 3 node identifiers (e.g. ["n1","n2","n3"]).
func NewCluster(nodeIDs []string) (*Cluster, error) {
	if len(nodeIDs) != 3 {
		return nil, fmt.Errorf("NewCluster: exactly 3 nodes required, got %d", len(nodeIDs))
	}

	c := &Cluster{
		nodes:   make(map[string]*node, 3),
		nodeIDs: nodeIDs,
		crashed: make(map[string]bool, 3),
	}

	// Phase 1: create all nodes (transport + raft instance).
	for _, id := range nodeIDs {
		n, err := newNode(id)
		if err != nil {
			return nil, fmt.Errorf("creating node %s: %w", id, err)
		}
		c.nodes[id] = n
	}

	// Phase 2: wire transports together so every node can reach every other.
	for _, id := range nodeIDs {
		for _, peer := range nodeIDs {
			if peer != id {
				c.nodes[id].transport.Connect(
					hraft.ServerAddress(peer),
					c.nodes[peer].transport,
				)
			}
		}
	}

	// Phase 3: bootstrap the cluster (only once, from the first node).
	servers := make([]hraft.Server, len(nodeIDs))
	for i, id := range nodeIDs {
		servers[i] = hraft.Server{
			ID:      hraft.ServerID(id),
			Address: hraft.ServerAddress(id),
		}
	}
	cfg := hraft.Configuration{Servers: servers}
	if err := c.nodes[nodeIDs[0]].raft.BootstrapCluster(cfg).Error(); err != nil {
		return nil, fmt.Errorf("bootstrapping cluster: %w", err)
	}

	return c, nil
}

// newNode builds the per-node config, stores, transport, and raft instance.
func newNode(id string) (*node, error) {
	cfg := hraft.DefaultConfig()
	cfg.LocalID = hraft.ServerID(id)
	cfg.HeartbeatTimeout = electionTimeout
	cfg.ElectionTimeout = electionTimeout
	cfg.LeaderLeaseTimeout = electionTimeout * 3 / 4
	cfg.CommitTimeout = 5 * time.Millisecond

	// Silence the raft logger in tests.
	cfg.LogLevel = "ERROR"

	logStore := hraft.NewInmemStore()
	stableStore := hraft.NewInmemStore()
	snapStore := hraft.NewInmemSnapshotStore()

	_, transport := hraft.NewInmemTransport(hraft.ServerAddress(id))

	fsm := NewSimpleFSM()

	r, err := hraft.NewRaft(cfg, fsm, logStore, stableStore, snapStore, transport)
	if err != nil {
		return nil, err
	}

	return &node{raft: r, transport: transport, fsm: fsm}, nil
}

// WaitForLeader blocks until a leader is elected or the timeout expires.
func (c *Cluster) WaitForLeader(timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if id := c.leaderID(); id != "" {
			return id, nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return "", fmt.Errorf("no leader elected within %v", timeout)
}

// leaderID returns the ID of the current leader, or "" if none.
func (c *Cluster) leaderID() string {
	for _, id := range c.nodeIDs {
		if !c.crashed[id] && c.nodes[id].raft.State() == hraft.Leader {
			return id
		}
	}
	return ""
}

// LeaderNode returns the current leader node, or nil if none.
func (c *Cluster) LeaderNode() *node {
	id := c.leaderID()
	if id == "" {
		return nil
	}
	return c.nodes[id]
}

// CountLeaders returns the number of nodes currently in Leader state.
func (c *Cluster) CountLeaders() int {
	count := 0
	for _, id := range c.nodeIDs {
		if !c.crashed[id] && c.nodes[id].raft.State() == hraft.Leader {
			count++
		}
	}
	return count
}

// CrashNode simulates a crash by fully disconnecting the node from the cluster
// network.  The remaining nodes will detect the missing heartbeats and elect
// a replacement leader.
func (c *Cluster) CrashNode(id string) error {
	if c.crashed[id] {
		return nil // already crashed
	}

	// Disconnect the crashing node from all its peers.
	c.nodes[id].transport.DisconnectAll()

	// Also disconnect peers from the crashing node so they stop sending to it.
	for _, peer := range c.nodeIDs {
		if peer != id {
			c.nodes[peer].transport.Disconnect(hraft.ServerAddress(id))
		}
	}

	c.crashed[id] = true
	return nil
}

// Stop gracefully shuts down all raft nodes.
func (c *Cluster) Stop() {
	for _, n := range c.nodes {
		n.raft.Shutdown().Error() //nolint:errcheck
	}
}
