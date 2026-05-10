package connector

import (
	"testing"

	itf "github.com/informalsystems/itf-go/itf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Tests ported from connect/src/driver/nondet.rs ---

func TestNewNondetPicks_FailOnNonRecord(t *testing.T) {
	expr := itf.Expr{Value: float64(42)}
	_, err := newNondetPicks(expr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "record")
}

func TestGetNondetPick_Found(t *testing.T) {
	option := itf.MapExprType{
		"tag":   {Value: "Some"},
		"value": {Value: float64(42)},
	}
	rec := itf.MapExprType{
		"foo": {Value: option},
	}
	picks, err := newNondetPicks(itf.Expr{Value: rec})
	require.NoError(t, err)

	got, ok := picks.Get("foo")
	assert.True(t, ok, "should find nondet value")
	assert.Equal(t, float64(42), got.Value)
}

func TestGetNondetPick_None(t *testing.T) {
	option := itf.MapExprType{
		"tag": {Value: "None"},
	}
	rec := itf.MapExprType{
		"foo": {Value: option},
	}
	picks, err := newNondetPicks(itf.Expr{Value: rec})
	require.NoError(t, err)

	_, ok := picks.Get("foo")
	assert.False(t, ok, "None variant should be filtered out")
}

func TestGetNondetPick_NonOption(t *testing.T) {
	// Non-option values should be returned as-is.
	rec := itf.MapExprType{
		"bar": {Value: "hello"},
	}
	picks, err := newNondetPicks(itf.Expr{Value: rec})
	require.NoError(t, err)

	got, ok := picks.Get("bar")
	assert.True(t, ok)
	assert.Equal(t, "hello", got.Value)
}

func TestGetNondetPick_GetString(t *testing.T) {
	rec := itf.MapExprType{
		"name": {Value: "alice"},
	}
	picks, err := newNondetPicks(itf.Expr{Value: rec})
	require.NoError(t, err)

	s, ok := picks.GetString("name")
	assert.True(t, ok)
	assert.Equal(t, "alice", s)
}

func TestGetNondetPick_GetInt(t *testing.T) {
	rec := itf.MapExprType{
		"count": {Value: float64(7)},
	}
	picks, err := newNondetPicks(itf.Expr{Value: rec})
	require.NoError(t, err)

	n, ok := picks.GetInt("count")
	assert.True(t, ok)
	assert.Equal(t, int64(7), n)
}

func TestGetNondetPick_GetBool(t *testing.T) {
	rec := itf.MapExprType{
		"flag": {Value: true},
	}
	picks, err := newNondetPicks(itf.Expr{Value: rec})
	require.NoError(t, err)

	b, ok := picks.GetBool("flag")
	assert.True(t, ok)
	assert.True(t, b)
}

// --- Tests ported from connect/src/value/option.rs ---

func TestUnwrapOption_SomeWithValue(t *testing.T) {
	rec := itf.MapExprType{
		"tag":   {Value: "Some"},
		"value": {Value: float64(42)},
	}
	value, present := unwrapOption(itf.Expr{Value: rec})
	assert.True(t, present)
	assert.Equal(t, float64(42), value.Value)
}

func TestUnwrapOption_None(t *testing.T) {
	rec := itf.MapExprType{
		"tag": {Value: "None"},
	}
	_, present := unwrapOption(itf.Expr{Value: rec})
	assert.False(t, present)
}

func TestUnwrapOption_NonOptionRecord(t *testing.T) {
	rec := itf.MapExprType{
		"foo": {Value: float64(42)},
		"bar": {Value: true},
	}
	value, present := unwrapOption(itf.Expr{Value: rec})
	assert.True(t, present)
	got, ok := value.Value.(itf.MapExprType)
	require.True(t, ok)
	assert.Equal(t, float64(42), got["foo"].Value)
	assert.Equal(t, true, got["bar"].Value)
}

func TestUnwrapOption_RecordWithNonStringTag(t *testing.T) {
	rec := itf.MapExprType{
		"tag":   {Value: float64(42)},
		"value": {Value: "test"},
	}
	value, present := unwrapOption(itf.Expr{Value: rec})
	assert.True(t, present)
	// Should be returned as-is since tag is not a string.
	got, ok := value.Value.(itf.MapExprType)
	require.True(t, ok)
	assert.Equal(t, float64(42), got["tag"].Value)
}

func TestUnwrapOption_NonRecord(t *testing.T) {
	// Non-record values are returned as-is.
	value, present := unwrapOption(itf.Expr{Value: "hello"})
	assert.True(t, present)
	assert.Equal(t, "hello", value.Value)
}
