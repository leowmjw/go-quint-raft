package connector

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"testing"

	itf "github.com/informalsystems/itf-go/itf"
)

// RunSimulation runs model-based tests by generating traces using "quint run"
// in simulation mode and replaying them through the given driver.
//
// If the driver also implements [CheckedDriver], CheckState is called after
// each step to verify implementation state against the specification.
//
// The test fails if:
//   - Quint is not in PATH or the command fails
//   - No traces are generated
//   - An anonymous action is encountered (name all step actions)
//   - [Driver.Step] returns an error
//   - [CheckedDriver.CheckState] returns an error
func RunSimulation(t testing.TB, driver Driver, config RunConfig) {
	t.Helper()

	seed := config.Seed
	if seed == "" {
		seed = genSeed()
		config.Seed = seed
	}

	logTitle("Running model-based tests for %s", t.Name())
	n := config.MaxSamples
	if n == 0 {
		n = defaultMaxSamples
	}
	logInfo("Generating %d traces using %q as random seed ...", n, seed)

	tmpDir := t.TempDir()
	outPattern := filepath.Join(tmpDir, "run_{seq}.itf.json")
	cmd := buildRunCommand(config, outPattern)

	traces, err := generateTraces(cmd, tmpDir)
	if err != nil {
		logError("%s", t.Name())
		logError("Reproduce this error with QUINT_SEED=%s", seed)
		t.Fatalf("generating traces: %v", err)
	}

	if err := replayTraces(t, driver, traces, seed); err != nil {
		logError("%s", t.Name())
		logError("Reproduce this error with QUINT_SEED=%s", seed)
		t.Fatalf("%v", err)
	}

	logSuccess("%s", t.Name())
}

// RunTest runs model-based tests by generating traces using "quint test" for a
// specific named test and replaying them through the given driver.
//
// If the driver also implements [CheckedDriver], CheckState is called after
// each step to verify implementation state against the specification.
func RunTest(t testing.TB, driver Driver, config TestConfig) {
	t.Helper()

	seed := config.Seed
	if seed == "" {
		seed = genSeed()
		config.Seed = seed
	}

	logTitle("Running model-based tests for %s", t.Name())
	n := config.MaxSamples
	if n == 0 {
		n = defaultMaxSamples
	}
	logInfo("Generating %d traces using %q as random seed ...", n, seed)

	tmpDir := t.TempDir()
	outPattern := filepath.Join(tmpDir, "test_{seq}.itf.json")
	cmd := buildTestCommand(config, outPattern)

	traces, err := generateTraces(cmd, tmpDir)
	if err != nil {
		logError("%s", t.Name())
		logError("Reproduce this error with QUINT_SEED=%s", seed)
		t.Fatalf("generating traces: %v", err)
	}

	if err := replayTraces(t, driver, traces, seed); err != nil {
		logError("%s", t.Name())
		logError("Reproduce this error with QUINT_SEED=%s", seed)
		t.Fatalf("%v", err)
	}

	logSuccess("%s", t.Name())
}

// replayTraces replays each ITF trace through the driver and optionally checks
// the state after each step.
func replayTraces(t testing.TB, driver Driver, tracePaths []string, seed string) error {
	t.Helper()

	if len(tracePaths) == 0 {
		return fmt.Errorf(
			"trace generation produced zero traces\n" +
				"please check your specification and/or your test configuration")
	}

	logInfo("Replaying traces ...")
	conf := driver.Config()
	checked, isChecked := driver.(CheckedDriver)

	for i, path := range tracePaths {
		logTrace(1, "[Trace %d]", i+1)

		trace, err := loadTrace(path)
		if err != nil {
			return fmt.Errorf("trace %d: %w", i+1, err)
		}

		// Copy vars to avoid modifying the trace when we delete mbt:: keys.
		for s, state := range trace.States {
			if state == nil {
				continue
			}
			vars := copyVars(state.VarValues)

			logTrace(2, "Deriving step from state %d ...", s)

			step, err := newStep(vars, conf)
			if err != nil {
				return fmt.Errorf("trace %d step %d: deriving step: %w", i+1, s, err)
			}
			logTrace(1, "[Step %d] action=%s nondet=%s", s, step.ActionTaken, step.NondetPicks.String())

			if step.ActionTaken == "" {
				return fmt.Errorf(
					"trace %d step %d: anonymous action found\n"+
						"please make sure all actions in step are properly named", i+1, s)
			}

			if err := driver.Step(step); err != nil {
				return fmt.Errorf("trace %d step %d (%s): %w", i+1, s, step.ActionTaken, err)
			}

			if isChecked {
				if err := checked.CheckState(step.State); err != nil {
					return fmt.Errorf("trace %d step %d (%s): state mismatch: %w", i+1, s, step.ActionTaken, err)
				}
			}
		}
	}
	return nil
}

// copyVars makes a shallow copy of a variable map so that deletions (of
// mbt:: keys) do not mutate the original parsed trace.
func copyVars(vars map[string]*itf.Expr) map[string]*itf.Expr {
	cp := make(map[string]*itf.Expr, len(vars))
	maps.Copy(cp, vars)
	return cp
}

// SkipIfNoQuint skips the test if the quint CLI is not available.
func SkipIfNoQuint(t testing.TB) {
	t.Helper()
	if _, err := findQuint(); err != nil {
		t.Skipf("quint CLI not found in PATH: %v", err)
	}
}

// findQuint locates the quint binary in PATH.
func findQuint() (string, error) {
	path, err := findExecutable("quint")
	if err != nil {
		return "", fmt.Errorf("quint not found in PATH: %w", err)
	}
	return path, nil
}

// findExecutable searches PATH for an executable with the given name.
func findExecutable(name string) (string, error) {
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return "", fmt.Errorf("PATH is empty")
	}
	for _, dir := range filepath.SplitList(pathEnv) {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", fmt.Errorf("%s not found", name)
}
