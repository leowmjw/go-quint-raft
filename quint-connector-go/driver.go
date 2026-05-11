package connector

// DriverConfig configures a [Driver] by specifying where to find the state
// and nondeterministic picks within a Quint specification's state space.
//
// By default both paths are empty, which means:
//   - State is extracted from the top level of the specification's state space
//   - Nondeterministic picks are extracted from Quint's builtin
//     mbt::actionTaken and mbt::nondetPicks variables
//
// Override these when your specification nests the relevant state within a
// larger structure, or when tracking nondeterminism manually via a sum type.
type DriverConfig struct {
	// StatePath is the path to navigate to reach the state to compare.
	// An empty slice means the state is at the top level.
	StatePath []string

	// NondetPath is the path to navigate to reach the nondeterministic picks.
	// An empty slice means to use Quint's builtin mbt::actionTaken and
	// mbt::nondetPicks variables.
	NondetPath []string
}

// Driver is the core interface for connecting Go implementations to Quint
// specifications.
//
// Implementations define how to execute steps from a Quint trace against the
// Go implementation. The framework automatically generates traces from your
// Quint specification and replays them through your driver.
//
// The Step method should use a switch on [Step.ActionTaken] and extract
// nondeterministic choices with [NondetPicks].
type Driver interface {
	// Step processes a single step from a Quint trace.
	Step(step *Step) error

	// Config returns the driver's configuration.
	Config() DriverConfig
}

// CheckedDriver extends [Driver] with state validation capabilities.
//
// After each step is executed the runner calls CheckState with the Quint
// specification's state from the trace. Implementations should compare
// the implementation's current state against the specification's state
// and return an error if they diverge.
//
// Use [ExprToMap] helpers to extract typed values from the ITF expression.
type CheckedDriver interface {
	Driver

	// CheckState validates that the implementation state matches the
	// specification state after the most recent Step call.
	// state is the Quint state expression narrowed by [DriverConfig.StatePath].
	CheckState(state ExprValue) error
}
