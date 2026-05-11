package multigroupraft

import (
	"context"
	"fmt"
	"time"

	dragonboat "github.com/lni/dragonboat/v4"
	"github.com/lni/dragonboat/v4/config"
	"github.com/lni/dragonboat/v4/logger"
)

// ShardIDs enumerates the two dragonboat Raft groups modelled by the spec.
var ShardIDs = []uint64{1, 2}

const (
	replicaID    = uint64(1)   // single-replica per shard (one NodeHost)
	rttMs        = uint64(5)   // very low RTT for local testing
	electionRTT  = uint64(10)  // election timeout in RTT units
	heartbeatRTT = uint64(1)   // heartbeat interval in RTT units

	waitLeaderTimeout  = 10 * time.Second
	waitLeaderInterval = 20 * time.Millisecond

	proposeTimeout = 3 * time.Second
)

// Cluster wraps a single dragonboat NodeHost that hosts multiple Raft shards.
// Each shard is an independent Raft group with its own KVStateMachine.
type Cluster struct {
	nh      *dragonboat.NodeHost
	dataDir string
	addr    string
}

// NewCluster creates and starts a Cluster.  dataDir is used for WAL/snapshot
// storage (use t.TempDir() in tests).  addr is the RaftAddress for the
// NodeHost (e.g. "localhost:19600").
func NewCluster(dataDir, addr string) (*Cluster, error) {
	// Silence dragonboat's verbose logs in tests.
	logger.GetLogger("raft").SetLevel(logger.WARNING)
	logger.GetLogger("rsm").SetLevel(logger.WARNING)
	logger.GetLogger("transport").SetLevel(logger.WARNING)
	logger.GetLogger("dragonboat").SetLevel(logger.WARNING)
	logger.GetLogger("logdb").SetLevel(logger.WARNING)
	logger.GetLogger("grpc").SetLevel(logger.WARNING)

	nhCfg := config.NodeHostConfig{
		WALDir:         dataDir,
		NodeHostDir:    dataDir,
		RTTMillisecond: rttMs,
		RaftAddress:    addr,
	}
	nh, err := dragonboat.NewNodeHost(nhCfg)
	if err != nil {
		return nil, fmt.Errorf("creating NodeHost: %w", err)
	}

	c := &Cluster{nh: nh, dataDir: dataDir, addr: addr}

	for _, shardID := range ShardIDs {
		rc := config.Config{
			ShardID:      shardID,
			ReplicaID:    replicaID,
			ElectionRTT:  electionRTT,
			HeartbeatRTT: heartbeatRTT,
			CheckQuorum:  true,
		}
		// Single replica: the initial-members map points only to this node.
		peers := map[uint64]string{replicaID: addr}
		if err := nh.StartReplica(peers, false, NewKVStateMachine, rc); err != nil {
			nh.Close()
			return nil, fmt.Errorf("starting shard %d: %w", shardID, err)
		}
	}

	return c, nil
}

// Close shuts the NodeHost down.
func (c *Cluster) Close() {
	if c.nh != nil {
		c.nh.Close()
		c.nh = nil
	}
}

// WaitForLeader blocks until a leader has been elected for shardID or until
// the timeout is reached.  It returns the replicaID of the elected leader.
func (c *Cluster) WaitForLeader(shardID uint64) (uint64, error) {
	deadline := time.Now().Add(waitLeaderTimeout)
	for time.Now().Before(deadline) {
		leaderID, _, ok, err := c.nh.GetLeaderID(shardID)
		if err == nil && ok {
			return leaderID, nil
		}
		time.Sleep(waitLeaderInterval)
	}
	return 0, fmt.Errorf("shard %d: no leader elected within %s", shardID, waitLeaderTimeout)
}

// HasLeader reports whether shardID currently has an elected leader.
func (c *Cluster) HasLeader(shardID uint64) bool {
	_, _, ok, err := c.nh.GetLeaderID(shardID)
	return err == nil && ok
}

// LeaderCount returns the number of shards that currently have an elected leader.
func (c *Cluster) LeaderCount() int {
	n := 0
	for _, shardID := range ShardIDs {
		if c.HasLeader(shardID) {
			n++
		}
	}
	return n
}

// Propose submits a no-op command to shardID and waits for it to be committed.
func (c *Cluster) Propose(shardID uint64) error {
	ctx, cancel := context.WithTimeout(context.Background(), proposeTimeout)
	defer cancel()

	session := c.nh.GetNoOPSession(shardID)
	cmd := []byte("ping") // minimal command; KVStateMachine ignores content
	_, err := c.nh.SyncPropose(ctx, session, cmd)
	if err != nil {
		return fmt.Errorf("shard %d propose: %w", shardID, err)
	}
	return nil
}

// Read queries the number of applied entries for shardID.
func (c *Cluster) Read(shardID uint64) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), proposeTimeout)
	defer cancel()

	result, err := c.nh.SyncRead(ctx, shardID, nil)
	if err != nil {
		return 0, fmt.Errorf("shard %d read: %w", shardID, err)
	}
	count, ok := result.(uint64)
	if !ok {
		return 0, fmt.Errorf("shard %d: unexpected read result type %T", shardID, result)
	}
	return count, nil
}
