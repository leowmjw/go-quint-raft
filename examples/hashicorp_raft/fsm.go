package hashicorpraft

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	hraft "github.com/hashicorp/raft"
)

// SimpleFSM is a key-value store that implements the hashicorp/raft FSM
// interface.  Each committed log entry must be a JSON-encoded Command.
type SimpleFSM struct {
	mu    sync.RWMutex
	store map[string]string
}

// Command is the payload applied to the FSM.
type Command struct {
	Op    string `json:"op"`    // "set"
	Key   string `json:"key"`
	Value string `json:"value"`
}

// NewSimpleFSM creates an empty FSM.
func NewSimpleFSM() *SimpleFSM {
	return &SimpleFSM{store: make(map[string]string)}
}

// Apply is called by the raft library once a log entry is committed.
// It decodes the entry as a Command and updates the in-memory store.
func (f *SimpleFSM) Apply(l *hraft.Log) interface{} {
	var cmd Command
	if err := json.Unmarshal(l.Data, &cmd); err != nil {
		return fmt.Errorf("fsm apply: decode: %w", err)
	}
	if cmd.Op != "set" {
		return fmt.Errorf("fsm apply: unknown op %q", cmd.Op)
	}
	f.mu.Lock()
	f.store[cmd.Key] = cmd.Value
	f.mu.Unlock()
	return nil
}

// Snapshot returns a point-in-time snapshot of the FSM for log compaction.
func (f *SimpleFSM) Snapshot() (hraft.FSMSnapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	copy := make(map[string]string, len(f.store))
	for k, v := range f.store {
		copy[k] = v
	}
	return &fsmSnapshot{store: copy}, nil
}

// Restore replaces the FSM state from a snapshot produced by Snapshot.
func (f *SimpleFSM) Restore(rc io.ReadCloser) error {
	defer rc.Close()
	store := make(map[string]string)
	if err := json.NewDecoder(rc).Decode(&store); err != nil {
		return fmt.Errorf("fsm restore: %w", err)
	}
	f.mu.Lock()
	f.store = store
	f.mu.Unlock()
	return nil
}

// Get returns the value for key, and whether it was found.
func (f *SimpleFSM) Get(key string) (string, bool) {
	f.mu.RLock()
	v, ok := f.store[key]
	f.mu.RUnlock()
	return v, ok
}

// CopyStore returns a copy of the current store contents.
func (f *SimpleFSM) CopyStore() map[string]string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	copy := make(map[string]string, len(f.store))
	for k, v := range f.store {
		copy[k] = v
	}
	return copy
}

// ---- FSMSnapshot ----

type fsmSnapshot struct {
	store map[string]string
}

func (s *fsmSnapshot) Persist(sink hraft.SnapshotSink) error {
	if err := json.NewEncoder(sink).Encode(s.store); err != nil {
		sink.Cancel() //nolint:errcheck
		return fmt.Errorf("snapshot persist: %w", err)
	}
	return sink.Close()
}

func (s *fsmSnapshot) Release() {}

// ---- encode/decode helpers ----

// encodeCommand serialises a Command to JSON bytes for raft.Apply.
func encodeCommand(key, value string) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(Command{Op: "set", Key: key, Value: value}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
