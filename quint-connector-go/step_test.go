package connector

import (
	"testing"

	itf "github.com/informalsystems/itf-go/itf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Tests ported from connect/src/driver/step.rs ---

func TestExtractValueInPath_EmptyPath(t *testing.T) {
	rec := itf.MapExprType{
		"key": {Value: "value"},
	}
	result, err := extractValueInPath(itf.Expr{Value: rec}, nil)
	require.NoError(t, err)

	got, ok := result.Value.(itf.MapExprType)
	require.True(t, ok)
	assert.Equal(t, "value", got["key"].Value)
}

func TestExtractValueInPath_SingleLevel(t *testing.T) {
	rec := itf.MapExprType{
		"key": {Value: "value"},
	}
	result, err := extractValueInPath(itf.Expr{Value: rec}, []string{"key"})
	require.NoError(t, err)
	assert.Equal(t, "value", result.Value)
}

func TestExtractValueInPath_Nested(t *testing.T) {
	inner := itf.MapExprType{
		"inner_key": {Value: float64(42)},
	}
	outer := itf.MapExprType{
		"outer_key": {Value: inner},
	}
	result, err := extractValueInPath(itf.Expr{Value: outer}, []string{"outer_key", "inner_key"})
	require.NoError(t, err)
	assert.Equal(t, float64(42), result.Value)
}

func TestExtractValueInPath_MissingKey(t *testing.T) {
	rec := itf.MapExprType{
		"key": {Value: "value"},
	}
	_, err := extractValueInPath(itf.Expr{Value: rec}, []string{"missing"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing")
}

func TestExtractValueInPath_NonRecord(t *testing.T) {
	rec := itf.MapExprType{
		"key": {Value: "value"},
	}
	_, err := extractValueInPath(itf.Expr{Value: rec}, []string{"key", "nested"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-record")
}

func TestFindRecordInPath_EmptyPath(t *testing.T) {
	rec := itf.MapExprType{}
	result, err := findRecordInPath(itf.Expr{Value: rec}, nil)
	require.NoError(t, err)
	assert.Equal(t, rec, result)
}

func TestFindRecordInPath_SingleLevel(t *testing.T) {
	inner := itf.MapExprType{}
	outer := itf.MapExprType{
		"inner": {Value: inner},
	}
	result, err := findRecordInPath(itf.Expr{Value: outer}, []string{"inner"})
	require.NoError(t, err)
	assert.Equal(t, inner, result)
}

func TestFindRecordInPath_Nested(t *testing.T) {
	innermost := itf.MapExprType{}
	middle := itf.MapExprType{
		"innermost": {Value: innermost},
	}
	outer := itf.MapExprType{
		"middle": {Value: middle},
	}
	result, err := findRecordInPath(itf.Expr{Value: outer}, []string{"middle", "innermost"})
	require.NoError(t, err)
	assert.Equal(t, innermost, result)
}

func TestFindRecordInPath_MissingKey(t *testing.T) {
	rec := itf.MapExprType{}
	_, err := findRecordInPath(itf.Expr{Value: rec}, []string{"missing"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Record")
}

func TestFindRecordInPath_NonRecord(t *testing.T) {
	rec := itf.MapExprType{
		"key": {Value: "value"},
	}
	_, err := findRecordInPath(itf.Expr{Value: rec}, []string{"key"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Record")
}

func TestExtractActionFromMBTVar_Success(t *testing.T) {
	vars := map[string]*itf.Expr{
		"mbt::actionTaken": {Value: "TestAction"},
	}
	result, err := extractActionFromMBTVar(vars)
	require.NoError(t, err)
	assert.Equal(t, "TestAction", result)
	// The key should still be in the map (deletion happens at a higher level).
	_, stillThere := vars["mbt::actionTaken"]
	assert.True(t, stillThere)
}

func TestExtractActionFromMBTVar_Missing(t *testing.T) {
	vars := map[string]*itf.Expr{}
	_, err := extractActionFromMBTVar(vars)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mbt::actionTaken")
}

func TestExtractActionFromMBTVar_WrongType(t *testing.T) {
	vars := map[string]*itf.Expr{
		"mbt::actionTaken": {Value: float64(42)},
	}
	_, err := extractActionFromMBTVar(vars)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mbt::actionTaken")
}

func TestExtractNondetFromMBTVar_Success(t *testing.T) {
	nondetRec := itf.MapExprType{
		"pick1": {Value: float64(1)},
	}
	vars := map[string]*itf.Expr{
		"mbt::nondetPicks": {Value: nondetRec},
	}
	result, err := extractNondetFromMBTVar(vars)
	require.NoError(t, err)
	// pick1 is not a Quint option, so it should be returned as-is.
	got, ok := result.Get("pick1")
	assert.True(t, ok)
	assert.Equal(t, float64(1), got.Value)
}

func TestExtractNondetFromMBTVar_Missing(t *testing.T) {
	vars := map[string]*itf.Expr{}
	_, err := extractNondetFromMBTVar(vars)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mbt::nondetPicks")
}

func TestExtractActionFromSumType_Success(t *testing.T) {
	rec := itf.MapExprType{
		"tag": {Value: "ActionName"},
	}
	result, err := extractActionFromSumType(rec)
	require.NoError(t, err)
	assert.Equal(t, "ActionName", result)
}

func TestExtractActionFromSumType_MissingTag(t *testing.T) {
	rec := itf.MapExprType{}
	_, err := extractActionFromSumType(rec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sum type")
}

func TestExtractActionFromSumType_WrongType(t *testing.T) {
	rec := itf.MapExprType{
		"tag": {Value: float64(42)},
	}
	_, err := extractActionFromSumType(rec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sum type")
}

func TestExtractNondetFromSumType_EmptyTuple(t *testing.T) {
	rec := itf.MapExprType{
		"value": {Value: itf.ListExprType{}},
	}
	result, err := extractNondetFromSumType(rec)
	require.NoError(t, err)
	assert.True(t, result.IsEmpty())
}

func TestExtractNondetFromSumType_Record(t *testing.T) {
	nondetRec := itf.MapExprType{
		"foo": {Value: float64(1)},
	}
	rec := itf.MapExprType{
		"value": {Value: nondetRec},
	}
	result, err := extractNondetFromSumType(rec)
	require.NoError(t, err)
	got, ok := result.Get("foo")
	assert.True(t, ok)
	assert.Equal(t, float64(1), got.Value)
}

func TestExtractNondetFromSumType_Invalid(t *testing.T) {
	rec := itf.MapExprType{
		"value": {Value: "invalid"},
	}
	_, err := extractNondetFromSumType(rec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nondet picks")
}

func TestExtractFromMBTVars_Success(t *testing.T) {
	nondetRec := itf.MapExprType{
		"pick1": {Value: float64(1)},
	}
	stateVal := "state_value"
	vars := map[string]*itf.Expr{
		"mbt::actionTaken": {Value: "TestAction"},
		"mbt::nondetPicks": {Value: nondetRec},
		"state_var":        {Value: stateVal},
	}

	step, err := extractFromMBTVars(vars, []string{"state_var"})
	require.NoError(t, err)
	assert.Equal(t, "TestAction", step.ActionTaken)
	assert.Equal(t, stateVal, step.State.Value)
}

func TestExtractFromMBTVars_MissingAction(t *testing.T) {
	nondetRec := itf.MapExprType{}
	vars := map[string]*itf.Expr{
		"mbt::nondetPicks": {Value: nondetRec},
	}

	_, err := extractFromMBTVars(vars, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mbt::actionTaken")
}

func TestExtractFromSumType_Success(t *testing.T) {
	sumRec := itf.MapExprType{
		"tag":   {Value: "TestAction"},
		"value": {Value: itf.ListExprType{}},
	}
	stateVal := "state_value"
	vars := map[string]*itf.Expr{
		"sum_type":  {Value: sumRec},
		"state_var": {Value: stateVal},
	}

	step, err := extractFromSumType(vars, []string{"sum_type"}, []string{"state_var"})
	require.NoError(t, err)
	assert.Equal(t, "TestAction", step.ActionTaken)
	assert.Equal(t, stateVal, step.State.Value)
	assert.True(t, step.NondetPicks.IsEmpty())
}

func TestExtractFromSumType_RemovesMBTVars(t *testing.T) {
	sumRec := itf.MapExprType{
		"tag":   {Value: "TestAction"},
		"value": {Value: itf.ListExprType{}},
	}
	stateVal := "state_value"
	vars := map[string]*itf.Expr{
		"sum_type":         {Value: sumRec},
		"mbt::actionTaken": {Value: "OldAction"},
		"mbt::nondetPicks": {Value: itf.MapExprType{}},
		"state_var":        {Value: stateVal},
	}

	step, err := extractFromSumType(vars, []string{"sum_type"}, []string{"state_var"})
	require.NoError(t, err)
	// The sum type's tag takes precedence.
	assert.Equal(t, "TestAction", step.ActionTaken)
}
