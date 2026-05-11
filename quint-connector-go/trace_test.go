package connector

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- Tests ported from connect/src/trace/generator/run.rs ---

func basicRunConfig() RunConfig {
	return RunConfig{
		Spec: "foo.qnt",
		Seed: "42",
	}
}

func runConfigToString(config RunConfig) string {
	cmd := buildRunCommand(config, filepath.Join("tmpdir", "run_{seq}.itf.json"))
	return strings.Join(cmd.Args, " ")
}

func TestRunConfig_Basic(t *testing.T) {
	config := basicRunConfig()
	got := runConfigToString(config)
	assert.Equal(t,
		"quint run foo.qnt --seed 42 --max-samples 100 --n-traces 100 "+
			"--out-itf tmpdir/run_{seq}.itf.json --mbt --verbosity 0",
		got,
	)
}

func TestRunConfig_MainModule(t *testing.T) {
	config := basicRunConfig()
	config.Main = "simulation"
	got := runConfigToString(config)
	assert.Equal(t,
		"quint run foo.qnt --seed 42 --max-samples 100 --n-traces 100 "+
			"--out-itf tmpdir/run_{seq}.itf.json --mbt --verbosity 0 --main simulation",
		got,
	)
}

func TestRunConfig_InitAction(t *testing.T) {
	config := basicRunConfig()
	config.Init = "my_init"
	got := runConfigToString(config)
	assert.Equal(t,
		"quint run foo.qnt --seed 42 --max-samples 100 --n-traces 100 "+
			"--out-itf tmpdir/run_{seq}.itf.json --mbt --verbosity 0 --init my_init",
		got,
	)
}

func TestRunConfig_StepAction(t *testing.T) {
	config := basicRunConfig()
	config.StepAction = "my_step"
	got := runConfigToString(config)
	assert.Equal(t,
		"quint run foo.qnt --seed 42 --max-samples 100 --n-traces 100 "+
			"--out-itf tmpdir/run_{seq}.itf.json --mbt --verbosity 0 --step my_step",
		got,
	)
}

func TestRunConfig_MaxSamples(t *testing.T) {
	config := basicRunConfig()
	config.MaxSamples = 42
	got := runConfigToString(config)
	assert.Equal(t,
		"quint run foo.qnt --seed 42 --max-samples 42 --n-traces 42 "+
			"--out-itf tmpdir/run_{seq}.itf.json --mbt --verbosity 0",
		got,
	)
}

func TestRunConfig_MaxSteps(t *testing.T) {
	config := basicRunConfig()
	config.MaxSteps = 32
	got := runConfigToString(config)
	assert.Equal(t,
		"quint run foo.qnt --seed 42 --max-samples 100 --n-traces 100 "+
			"--out-itf tmpdir/run_{seq}.itf.json --mbt --verbosity 0 --max-steps 32",
		got,
	)
}

// --- Tests ported from connect/src/trace/generator/test.rs ---

func basicTestConfig() TestConfig {
	return TestConfig{
		Spec: "foo.qnt",
		Test: "happyTest",
		Seed: "42",
	}
}

func testConfigToString(config TestConfig) string {
	cmd := buildTestCommand(config, filepath.Join("tmpdir", "test_{seq}.itf.json"))
	return strings.Join(cmd.Args, " ")
}

func TestTestConfig_Basic(t *testing.T) {
	config := basicTestConfig()
	got := testConfigToString(config)
	assert.Equal(t,
		"quint test foo.qnt --seed 42 --match ^happyTest$ --max-samples 100 "+
			"--out-itf tmpdir/test_{seq}.itf.json --verbosity 0",
		got,
	)
}

func TestTestConfig_MainModule(t *testing.T) {
	config := basicTestConfig()
	config.Main = "tests"
	got := testConfigToString(config)
	assert.Equal(t,
		"quint test foo.qnt --seed 42 --match ^happyTest$ --max-samples 100 "+
			"--out-itf tmpdir/test_{seq}.itf.json --verbosity 0 --main tests",
		got,
	)
}

func TestTestConfig_MaxSamples(t *testing.T) {
	config := basicTestConfig()
	config.MaxSamples = 42
	got := testConfigToString(config)
	assert.Equal(t,
		"quint test foo.qnt --seed 42 --match ^happyTest$ --max-samples 42 "+
			"--out-itf tmpdir/test_{seq}.itf.json --verbosity 0",
		got,
	)
}
