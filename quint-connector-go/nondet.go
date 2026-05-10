package connector

import (
	"fmt"

	itf "github.com/informalsystems/itf-go/itf"
)

// ExprValue is an alias for the ITF Expr type, representing any value in the
// Informal Trace Format.  It is used for state comparison in [CheckedDriver].
type ExprValue = itf.Expr

// NondetPicks wraps the nondeterministic choices made during trace generation.
//
// Picks are extracted from the mbt::nondetPicks variable in the Quint trace
// (or from a custom sum type if configured via [DriverConfig.NondetPath]).
//
// Each pick corresponds to a nondet variable in the Quint specification.
// Values that are Quint None are automatically filtered out.
type NondetPicks struct {
	picks map[string]itf.Expr
}

// Get retrieves a nondeterministic pick by name.
// Returns the Expr value and true if found, or the zero Expr and false if not.
func (n *NondetPicks) Get(name string) (itf.Expr, bool) {
	expr, ok := n.picks[name]
	return expr, ok
}

// GetString retrieves a nondeterministic pick as a string.
// Returns the string value and true if found and the value is a string.
func (n *NondetPicks) GetString(name string) (string, bool) {
	expr, ok := n.picks[name]
	if !ok {
		return "", false
	}
	s, ok := expr.Value.(string)
	return s, ok
}

// GetInt retrieves a nondeterministic pick as an int64.
// Handles both float64 (JSON number) and int64 (bigint) representations.
func (n *NondetPicks) GetInt(name string) (int64, bool) {
	expr, ok := n.picks[name]
	if !ok {
		return 0, false
	}
	switch v := expr.Value.(type) {
	case int64:
		return v, true
	case float64:
		return int64(v), true
	}
	return 0, false
}

// GetBool retrieves a nondeterministic pick as a bool.
func (n *NondetPicks) GetBool(name string) (bool, bool) {
	expr, ok := n.picks[name]
	if !ok {
		return false, false
	}
	b, ok := expr.Value.(bool)
	return b, ok
}

// IsEmpty reports whether there are no nondeterministic picks.
func (n *NondetPicks) IsEmpty() bool {
	return len(n.picks) == 0
}

// String returns a human-readable representation of the picks.
func (n *NondetPicks) String() string {
	if n.IsEmpty() {
		return "<none>"
	}
	s := ""
	for k, v := range n.picks {
		if s != "" {
			s += "\n"
		}
		s += fmt.Sprintf("+ %s: %v", k, v.Value)
	}
	return s
}

// newNondetPicks builds a NondetPicks from an ITF map expression, filtering
// out Quint None variants so that only present choices are exposed.
func newNondetPicks(expr itf.Expr) (NondetPicks, error) {
	rec, ok := expr.Value.(itf.MapExprType)
	if !ok {
		return NondetPicks{}, fmt.Errorf("nondet picks must be a record, got %T", expr.Value)
	}
	picks := make(map[string]itf.Expr, len(rec))
	for key, val := range rec {
		inner, present := unwrapOption(val)
		if present {
			picks[key] = inner
		}
	}
	return NondetPicks{picks: picks}, nil
}

// emptyNondetPicks returns a NondetPicks with no entries.
func emptyNondetPicks() NondetPicks {
	return NondetPicks{picks: make(map[string]itf.Expr)}
}

// unwrapOption interprets a Quint option value.
//
// Quint serialises options as:
//   - { "tag": "Some", "value": x }  → (x, true)
//   - { "tag": "None", "value": [] } → (zero, false)
//
// Non-option records and all other values are returned as-is with present=true.
func unwrapOption(expr itf.Expr) (value itf.Expr, present bool) {
	rec, ok := expr.Value.(itf.MapExprType)
	if !ok {
		return expr, true
	}
	tagExpr, hasTag := rec["tag"]
	if !hasTag {
		return expr, true
	}
	tag, ok := tagExpr.Value.(string)
	if !ok {
		return expr, true
	}
	switch tag {
	case "None":
		return itf.Expr{}, false
	case "Some":
		valueExpr, hasValue := rec["value"]
		if !hasValue {
			return itf.Expr{}, false
		}
		return valueExpr, true
	default:
		return expr, true
	}
}
