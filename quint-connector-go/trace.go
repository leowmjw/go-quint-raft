package connector

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	itf "github.com/informalsystems/itf-go/itf"
)

// RunConfig configures trace generation using "quint run" in simulation mode.
//
// Quint's simulation mode randomly explores the specification by choosing
// nondeterministic values and generates one ITF trace file per sample.
type RunConfig struct {
	// Spec is the path to the Quint specification file (required).
	Spec string

	// Main is the name of the main module (optional; Quint's default if empty).
	Main string

	// Init is the name of the init action (optional; Quint's default if empty).
	Init string

	// StepAction is the name of the step action (optional; Quint's default if empty).
	StepAction string

	// MaxSamples is the maximum number of traces to generate (default: 100).
	MaxSamples int

	// MaxSteps is the maximum number of steps per trace (optional; Quint's default if empty).
	MaxSteps int

	// Seed is the random seed for reproducibility (auto-generated if empty).
	Seed string
}

// TestConfig configures trace generation using "quint test".
//
// This runs a specific named Quint test and generates ITF trace files.
type TestConfig struct {
	// Spec is the path to the Quint specification file (required).
	Spec string

	// Test is the name of the Quint test to run (required).
	Test string

	// Main is the name of the main module containing the test (optional).
	Main string

	// MaxSamples is the maximum number of test runs (default: 100).
	MaxSamples int

	// Seed is the random seed for reproducibility (auto-generated if empty).
	Seed string
}

const defaultMaxSamples = 100

// buildRunCommand creates the quint run command for the given config.
// outPattern is the output file pattern, e.g. "/tmp/dir/run_{seq}.itf.json".
func buildRunCommand(config RunConfig, outPattern string) *exec.Cmd {
	n := config.MaxSamples
	if n == 0 {
		n = defaultMaxSamples
	}
	seed := config.Seed
	if seed == "" {
		seed = genSeed()
	}
	nStr := fmt.Sprintf("%d", n)

	args := []string{
		"run", config.Spec,
		"--seed", seed,
		"--max-samples", nStr,
		"--n-traces", nStr,
		"--out-itf", outPattern,
		"--mbt",
		"--verbosity", "0",
	}
	if config.Main != "" {
		args = append(args, "--main", config.Main)
	}
	if config.Init != "" {
		args = append(args, "--init", config.Init)
	}
	if config.StepAction != "" {
		args = append(args, "--step", config.StepAction)
	}
	if config.MaxSteps > 0 {
		args = append(args, "--max-steps", fmt.Sprintf("%d", config.MaxSteps))
	}
	return exec.Command("quint", args...) //nolint:gosec // spec path is user-provided
}

// buildTestCommand creates the quint test command for the given config.
func buildTestCommand(config TestConfig, outPattern string) *exec.Cmd {
	n := config.MaxSamples
	if n == 0 {
		n = defaultMaxSamples
	}
	seed := config.Seed
	if seed == "" {
		seed = genSeed()
	}
	nStr := fmt.Sprintf("%d", n)

	args := []string{
		"test", config.Spec,
		"--seed", seed,
		"--match", fmt.Sprintf("^%s$", config.Test),
		"--max-samples", nStr,
		"--out-itf", outPattern,
		"--verbosity", "0",
	}
	if config.Main != "" {
		args = append(args, "--main", config.Main)
	}
	return exec.Command("quint", args...) //nolint:gosec // spec path is user-provided
}

// generateTraces runs a quint command and returns the paths of generated ITF trace files.
func generateTraces(cmd *exec.Cmd, tmpDir string) ([]string, error) {
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("quint command failed: %w\n%s", err, out)
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return nil, fmt.Errorf("listing trace files: %w", err)
	}
	var paths []string
	for _, e := range entries {
		if !e.IsDir() {
			paths = append(paths, filepath.Join(tmpDir, e.Name()))
		}
	}
	return paths, nil
}

// loadTrace reads and parses an ITF trace file.
func loadTrace(path string) (*itf.Trace, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening trace %s: %w", path, err)
	}
	defer f.Close()

	var trace itf.Trace
	if err := json.NewDecoder(f).Decode(&trace); err != nil {
		return nil, fmt.Errorf("parsing trace %s: %w", path, err)
	}
	return &trace, nil
}
