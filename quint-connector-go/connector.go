// Package connector is a Go port of the quint-connect Rust library.
//
// It provides a model-based testing framework that connects Quint specifications
// with Go applications. By automatically generating traces from Quint specifications
// and replaying them against Go implementations, you can verify that your code
// behaves exactly as specified.
//
// # Overview
//
// Quint Connect enables rigorous testing of Go code against formal specifications
// written in Quint. The workflow is:
//
//  1. Write a Quint specification describing your system's behaviour
//  2. Implement the [Driver] interface to replay specification steps against your code
//  3. Use [RunSimulation] or [RunTest] to generate and replay traces
//
// # Quick Start
//
// Implement the [Driver] interface:
//
//	type MyDriver struct { /* your state */ }
//
//	func (d *MyDriver) Step(step *connector.Step) error {
//	    switch step.ActionTaken {
//	    case "init":
//	        d.reset()
//	    case "myAction":
//	        param, _ := step.NondetPicks.GetString("param")
//	        d.doSomething(param)
//	    default:
//	        return fmt.Errorf("unknown action: %s", step.ActionTaken)
//	    }
//	    return nil
//	}
//
//	func (d *MyDriver) Config() connector.DriverConfig {
//	    return connector.DriverConfig{}
//	}
//
// Run the model-based test:
//
//	func TestMySystem(t *testing.T) {
//	    connector.RunSimulation(t, &MyDriver{}, connector.RunConfig{
//	        Spec:       "spec/my_spec.qnt",
//	        MaxSamples: 10,
//	    })
//	}
//
// # Verbosity
//
// Set the QUINT_VERBOSE environment variable to control output:
//   - QUINT_VERBOSE=0: minimal output (default)
//   - QUINT_VERBOSE=1: show trace and step information
//   - QUINT_VERBOSE=2: show detailed state derivation
//
// # Reproducibility
//
// Failed tests display the random seed used:
//
//	Reproduce this error with QUINT_SEED=42
//
// Set the seed to reproduce exact test traces:
//
//	QUINT_SEED=42 go test ./...
package connector
