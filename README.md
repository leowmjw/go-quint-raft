# go-quint-raft

Go port of Model-Based Testing with quint-connect — a framework for verifying
Go implementations against [Quint](https://github.com/informalsystems/quint)
formal specifications.

## Overview

This repository contains:

| Path | Description |
|------|-------------|
| [`quint-connector-go/`](./quint-connector-go) | Go port of [quint-connect](https://github.com/quint-co/quint-connect) (Rust) |
| [`examples/tictactoe/`](./examples/tictactoe) | Simplest example — TicTacToe game verification |
| [`examples/two_phase_commit/`](./examples/two_phase_commit) | Intermediate — two-phase commit protocol |
| [`examples/simple_raft/`](./examples/simple_raft) | Intermediate — simple in-memory Raft model inspired by [elijah0528/raft](https://github.com/elijah0528/raft) |
| [`examples/raft/`](./examples/raft) | Full example — Raft leader election using [raftly](https://github.com/ani03sha/raftly) |

## Quick Start

### Prerequisites

```sh
# Install mise (task runner + toolchain manager)
curl https://mise.run | sh

# Install Go 1.26 + Node.js (for quint CLI)
mise install

# Install the Quint CLI
mise run install-quint
```

### Running tests

```sh
# Unit tests only (no quint CLI required)
mise run test-unit

# All tests including model-based integration tests (requires quint)
mise run test

# Just the raft example unit test
GOTOOLCHAIN=auto go test ./examples/raft/... -run TestRaftLeaderElection_Unit -v
```

## quint-connector-go

A Go port of the [quint-connect](https://github.com/quint-co/quint-connect) Rust
library. It connects Quint specifications to Go implementations for model-based
testing.

### Key types

| Type | Description |
|------|-------------|
| `Driver` | Core interface — implement `Step(*Step) error` and `Config() DriverConfig` |
| `CheckedDriver` | Extends `Driver` with `CheckState(ExprValue) error` for state validation |
| `Step` | A single trace step with `ActionTaken` (string) and `NondetPicks` |
| `NondetPicks` | Nondeterministic choices: `.GetString()`, `.GetInt()`, `.GetBool()` |
| `DriverConfig` | Configures `StatePath` and `NondetPath` for nested spec navigation |
| `RunConfig` | Configuration for `RunSimulation` (uses `quint run`) |
| `TestConfig` | Configuration for `RunTest` (uses `quint test`) |

### Usage

```go
type MyDriver struct { /* your state */ }

func (d *MyDriver) Config() connector.DriverConfig {
    return connector.DriverConfig{} // default: top-level state, mbt:: vars
}

func (d *MyDriver) Step(step *connector.Step) error {
    switch step.ActionTaken {
    case "init":
        d.reset()
    case "myAction":
        param, _ := step.NondetPicks.GetString("param")
        d.doSomething(param)
    default:
        return fmt.Errorf("unknown action: %s", step.ActionTaken)
    }
    return nil
}

func TestMySystem(t *testing.T) {
    connector.SkipIfNoQuint(t) // skip gracefully if quint CLI not found
    connector.RunSimulation(t, &MyDriver{}, connector.RunConfig{
        Spec:       "spec/my_spec.qnt",
        MaxSamples: 10,
    })
}
```

## Examples

The examples form a progressive learning path:

### 1. TicTacToe (simplest)

- Basic `Driver` + `CheckedDriver` usage
- Default config (mbt:: variables, top-level state)
- Board and player state comparison

```sh
go test ./examples/tictactoe/... -v
```

### 2. Two-Phase Commit (intermediate)

- Custom `StatePath` and `NondetPath` for nested spec state
- Sum-type action dispatch via the Choreo framework
- Per-node stage verification

```sh
go test ./examples/two_phase_commit/... -v
```

### 3. Raft Leader Election (full)

- Real asynchronous distributed system ([raftly](https://github.com/ani03sha/raftly))
- Abstract Quint spec maps to real cluster operations
- ElectionSafety invariant: at most one leader per term

```sh
GOTOOLCHAIN=auto go test ./examples/raft/... -v
```

### 4. Simple Raft (leader election model)

- Lightweight model inspired by [elijah0528/raft](https://github.com/elijah0528/raft)
- Tracks node roles, terms, votes, and node liveness
- Checks ElectionSafety + at-most-one-leader invariants on every step

```sh
go test ./examples/simple_raft/... -v
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `QUINT_SEED` | Fix the random seed for reproducibility (e.g. `QUINT_SEED=42`) |
| `QUINT_VERBOSE` | Verbosity level: 0=minimal, 1=trace info, 2=detailed state |
| `GOTOOLCHAIN` | Set to `auto` when running raft example (requires Go 1.26) |

## Architecture

```
quint-connector-go/
├── connector.go   package documentation
├── driver.go      Driver + CheckedDriver interfaces, DriverConfig
├── nondet.go      NondetPicks, unwrapOption (Quint option → Go)
├── step.go        Step, extraction from ITF state (mbt:: or sum type)
├── runner.go      RunSimulation, RunTest, SkipIfNoQuint
├── trace.go       quint CLI command building, ITF trace loading
└── seed.go        random seed generation, log helpers
``` 
