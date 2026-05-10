package connector

import (
	"fmt"

	itf "github.com/informalsystems/itf-go/itf"
)

// Step represents a single step in a trace generated from a Quint specification.
//
// Steps are passed to [Driver.Step] for execution against the implementation.
// Use [Step.ActionTaken] to identify the action and [Step.NondetPicks] to
// access nondeterministic choices made during that action.
type Step struct {
	// ActionTaken is the name of the Quint action executed in this step.
	// It corresponds to mbt::actionTaken from the trace.
	ActionTaken string

	// NondetPicks contains the nondeterministic choices made during this step.
	// It corresponds to mbt::nondetPicks from the trace.
	NondetPicks NondetPicks

	// State is the Quint specification state after executing this step,
	// narrowed by [DriverConfig.StatePath]. It is used for state comparison
	// in [CheckedDriver.CheckState].
	State itf.Expr
}

// newStep constructs a Step from an ITF state variable map and driver config.
// It handles both the default mbt:: variable extraction and custom sum type
// extraction based on the config's NondetPath.
func newStep(vars map[string]*itf.Expr, config DriverConfig) (*Step, error) {
	if len(config.NondetPath) == 0 {
		return extractFromMBTVars(vars, config.StatePath)
	}
	return extractFromSumType(vars, config.NondetPath, config.StatePath)
}

// extractFromMBTVars extracts a Step using Quint's builtin mbt:: variables.
func extractFromMBTVars(vars map[string]*itf.Expr, statePath []string) (*Step, error) {
	action, err := extractActionFromMBTVar(vars)
	if err != nil {
		return nil, err
	}
	delete(vars, "mbt::actionTaken")

	nondet, err := extractNondetFromMBTVar(vars)
	if err != nil {
		return nil, err
	}
	delete(vars, "mbt::nondetPicks")

	state, err := extractValueInPath(varsToRecord(vars), statePath)
	if err != nil {
		return nil, err
	}

	return &Step{
		ActionTaken: action,
		NondetPicks: nondet,
		State:       state,
	}, nil
}

// extractFromSumType extracts a Step where nondeterminism is tracked via a
// custom sum type at the given path rather than mbt:: variables.
func extractFromSumType(vars map[string]*itf.Expr, sumTypePath, statePath []string) (*Step, error) {
	// Find the sum type record at the given path without modifying the state.
	sumType, err := findRecordInPath(varsToRecord(vars), sumTypePath)
	if err != nil {
		return nil, fmt.Errorf("finding sum type at %v: %w", sumTypePath, err)
	}

	action, err := extractActionFromSumType(sumType)
	if err != nil {
		return nil, err
	}

	nondet, err := extractNondetFromSumType(sumType)
	if err != nil {
		return nil, err
	}

	// Remove mbt variables if present (they may or may not exist).
	delete(vars, "mbt::actionTaken")
	delete(vars, "mbt::nondetPicks")

	state, err := extractValueInPath(varsToRecord(vars), statePath)
	if err != nil {
		return nil, err
	}

	return &Step{
		ActionTaken: action,
		NondetPicks: nondet,
		State:       state,
	}, nil
}

// extractActionFromMBTVar reads the action name from mbt::actionTaken.
func extractActionFromMBTVar(vars map[string]*itf.Expr) (string, error) {
	expr, ok := vars["mbt::actionTaken"]
	if !ok || expr == nil {
		return "", fmt.Errorf("missing mbt::actionTaken variable in trace")
	}
	action, ok := expr.Value.(string)
	if !ok {
		return "", fmt.Errorf("mbt::actionTaken is not a string, got %T", expr.Value)
	}
	return action, nil
}

// extractNondetFromMBTVar reads the nondeterministic picks from mbt::nondetPicks.
func extractNondetFromMBTVar(vars map[string]*itf.Expr) (NondetPicks, error) {
	expr, ok := vars["mbt::nondetPicks"]
	if !ok || expr == nil {
		return NondetPicks{}, fmt.Errorf("missing mbt::nondetPicks variable in trace")
	}
	return newNondetPicks(*expr)
}

// extractActionFromSumType reads the action name from a sum type record's tag field.
func extractActionFromSumType(rec itf.MapExprType) (string, error) {
	tagExpr, ok := rec["tag"]
	if !ok {
		return "", fmt.Errorf("expected action as sum type variant, missing 'tag' field; got: %v", rec)
	}
	tag, ok := tagExpr.Value.(string)
	if !ok {
		return "", fmt.Errorf("expected action to be a sum type variant with string tag, got %T; value: %v", tagExpr.Value, rec)
	}
	return tag, nil
}

// extractNondetFromSumType reads nondeterministic picks from a sum type record's value field.
func extractNondetFromSumType(rec itf.MapExprType) (NondetPicks, error) {
	valueExpr, ok := rec["value"]
	if !ok {
		return emptyNondetPicks(), nil
	}
	switch v := valueExpr.Value.(type) {
	case itf.ListExprType:
		if len(v) == 0 {
			// Empty tuple — no nondet picks.
			return emptyNondetPicks(), nil
		}
		return NondetPicks{}, fmt.Errorf("expected nondet picks to be a record or empty tuple, got non-empty list")
	case itf.MapExprType:
		picks := make(map[string]itf.Expr, len(v))
		for k, e := range v {
			picks[k] = e
		}
		return NondetPicks{picks: picks}, nil
	default:
		return NondetPicks{}, fmt.Errorf("expected nondet picks as sum type value to be a record or empty tuple, got %T; value: %v", valueExpr.Value, rec)
	}
}

// extractValueInPath navigates into an ITF expression following the given path
// segments, returning the expression at the end of the path.
//
// If path is empty, the original expression is returned unchanged.
// Returns an error if any path segment is missing or the current value is not
// a map/record.
func extractValueInPath(expr itf.Expr, path []string) (itf.Expr, error) {
	current := expr
	for _, segment := range path {
		rec, ok := current.Value.(itf.MapExprType)
		if !ok {
			return itf.Expr{}, fmt.Errorf("cannot read %q from non-record value in path %v", segment, path)
		}
		next, ok := rec[segment]
		if !ok {
			return itf.Expr{}, fmt.Errorf("cannot find value at %q in path %v; current value: %v", segment, path, current.Value)
		}
		current = next
	}
	return current, nil
}

// findRecordInPath navigates into an ITF expression following the given path
// segments, returning the map at the end of the path without consuming it.
func findRecordInPath(expr itf.Expr, path []string) (itf.MapExprType, error) {
	current, ok := expr.Value.(itf.MapExprType)
	if !ok {
		return nil, fmt.Errorf("root value is not a record; got %T", expr.Value)
	}
	for _, segment := range path {
		next, ok := current[segment]
		if !ok {
			return nil, fmt.Errorf("cannot find Record at %q in path %v", segment, path)
		}
		rec, ok := next.Value.(itf.MapExprType)
		if !ok {
			return nil, fmt.Errorf("cannot find Record at %q in path %v; value is %T", segment, path, next.Value)
		}
		current = rec
	}
	return current, nil
}

// varsToRecord converts an ITF variable map (with pointer values) to an ITF
// Expr whose Value is a MapExprType, suitable for path navigation.
func varsToRecord(vars map[string]*itf.Expr) itf.Expr {
	rec := make(itf.MapExprType, len(vars))
	for k, v := range vars {
		if v != nil {
			rec[k] = *v
		}
	}
	return itf.Expr{Value: rec}
}
