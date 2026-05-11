// Package multigroupraft implements a multi-group Raft example using dragonboat.
//
// It demonstrates model-based testing of a two-shard Raft cluster against a Quint
// specification.  Each shard runs as an independent dragonboat Raft group on a
// single NodeHost.  The KV state machine below is used as the replicated state
// for both shards.
package multigroupraft

import (
	"encoding/binary"
	"io"
	"sync"

	sm "github.com/lni/dragonboat/v4/statemachine"
)

// KVStateMachine is a simple in-memory key/value store that satisfies the
// dragonboat sm.IStateMachine interface.  It is used as the replicated state
// machine for every shard in the example cluster.
//
// Commands are 8-byte big-endian encoded uint64 values.  The value is stored
// under the key equal to the shard ID.  This keeps the implementation minimal
// while still exercising the full Raft path.
type KVStateMachine struct {
	mu      sync.RWMutex
	shardID uint64
	count   uint64 // number of applied entries
}

// NewKVStateMachine is the factory function passed to dragonboat's StartReplica.
func NewKVStateMachine(shardID, _ uint64) sm.IStateMachine {
	return &KVStateMachine{shardID: shardID}
}

// Update applies a single log entry to the state machine.
func (k *KVStateMachine) Update(e sm.Entry) (sm.Result, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.count++
	return sm.Result{Value: k.count}, nil
}

// Lookup returns the number of applied entries.
func (k *KVStateMachine) Lookup(query interface{}) (interface{}, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.count, nil
}

// SaveSnapshot serialises the counter to the writer.
func (k *KVStateMachine) SaveSnapshot(w io.Writer, _ sm.ISnapshotFileCollection, _ <-chan struct{}) error {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return binary.Write(w, binary.BigEndian, k.count)
}

// RecoverFromSnapshot restores the counter from a snapshot.
func (k *KVStateMachine) RecoverFromSnapshot(r io.Reader, _ []sm.SnapshotFile, _ <-chan struct{}) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	return binary.Read(r, binary.BigEndian, &k.count)
}

// Close is a no-op for an in-memory state machine.
func (k *KVStateMachine) Close() error { return nil }
