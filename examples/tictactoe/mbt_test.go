package tictactoe

import (
	"fmt"
	"testing"

	connector "github.com/leowmjw/go-quint-raft/quint-connector-go"
	itf "github.com/informalsystems/itf-go/itf"
)

// ---- State type (matches Quint spec variables board and nextTurn) ----

// specPlayer is a Quint Player sum type: {tag: "X"} or {tag: "O"}.
type specPlayer int

func playerFromExpr(e itf.Expr) (specPlayer, error) {
	rec, ok := e.Value.(itf.MapExprType)
	if !ok {
		return 0, fmt.Errorf("player: expected record, got %T", e.Value)
	}
	tagExpr, ok := rec["tag"]
	if !ok {
		return 0, fmt.Errorf("player: missing tag")
	}
	tag, ok := tagExpr.Value.(string)
	if !ok {
		return 0, fmt.Errorf("player: tag not string")
	}
	switch tag {
	case "X":
		return specPlayer(PlayerX), nil
	case "O":
		return specPlayer(PlayerO), nil
	default:
		return 0, fmt.Errorf("player: unknown tag %q", tag)
	}
}

// ---- Driver ----

// TicTacToeDriver connects the Go TicTacToe game to the Quint specification.
// It is a direct Go port of the Rust TicTacToeDriver in mbt.rs.
type TicTacToeDriver struct {
	game TicTacToe
}

func (d *TicTacToeDriver) Config() connector.DriverConfig {
	return connector.DriverConfig{}
}

// Step processes a single Quint trace step.
// Action names match the Quint spec: init, MoveX, MoveO, stuttered.
func (d *TicTacToeDriver) Step(step *connector.Step) error {
	switch step.ActionTaken {
	case "init":
		d.game = TicTacToe{NextTurn: PlayerX}

	case "MoveX":
		// The spec uses nondet picks named `corner` or `coordinate`.
		pos, err := positionFromNondet(step, "corner", "coordinate")
		if err != nil {
			// Fall back to (1,1) when neither pick is available.
			pos = Position{Col: 1, Row: 1}
		}
		d.game.MoveTo(pos, PlayerX)

	case "MoveO":
		pos, err := positionFromNondet(step, "coordinate", "")
		if err != nil {
			return fmt.Errorf("MoveO: %w", err)
		}
		d.game.MoveTo(pos, PlayerO)

	case "stuttered":
		// Game over – no action needed.

	default:
		return fmt.Errorf("unknown action: %s", step.ActionTaken)
	}
	return nil
}

// CheckState validates the implementation board and nextTurn against the spec.
func (d *TicTacToeDriver) CheckState(specState connector.ExprValue) error {
	rec, ok := specState.Value.(itf.MapExprType)
	if !ok {
		return fmt.Errorf("state: expected record, got %T", specState.Value)
	}

	// Verify nextTurn.
	if ntExpr, ok := rec["nextTurn"]; ok {
		sp, err := playerFromExpr(ntExpr)
		if err != nil {
			return fmt.Errorf("nextTurn: %w", err)
		}
		if specPlayer(d.game.NextTurn) != sp {
			return fmt.Errorf("nextTurn mismatch: impl=%v spec=%v", d.game.NextTurn, sp)
		}
	}

	// Verify board.
	if boardExpr, ok := rec["board"]; ok {
		if err := d.checkBoard(boardExpr); err != nil {
			return err
		}
	}
	return nil
}

func (d *TicTacToeDriver) checkBoard(boardExpr itf.Expr) error {
	// Quint board: int -> (int -> Square)
	// In ITF: MapExprType where keys are string-ified column ints.
	cols, ok := boardExpr.Value.(itf.MapExprType)
	if !ok {
		return fmt.Errorf("board: expected map, got %T", boardExpr.Value)
	}
	for colKey, rowsExpr := range cols {
		var colIdx int
		if _, err := fmt.Sscanf(colKey, "%d", &colIdx); err != nil {
			return fmt.Errorf("board: bad column key %q", colKey)
		}
		rows, ok := rowsExpr.Value.(itf.MapExprType)
		if !ok {
			return fmt.Errorf("board[%d]: expected map", colIdx)
		}
		for rowKey, squareExpr := range rows {
			var rowIdx int
			if _, err := fmt.Sscanf(rowKey, "%d", &rowIdx); err != nil {
				return fmt.Errorf("board[%d]: bad row key %q", colIdx, rowKey)
			}
			if err := d.checkSquare(colIdx, rowIdx, squareExpr); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *TicTacToeDriver) checkSquare(col, row int, squareExpr itf.Expr) error {
	// Square is a sum type: {tag: "Empty"} or {tag: "Occupied", value: {tag: "X"|"O"}}
	rec, ok := squareExpr.Value.(itf.MapExprType)
	if !ok {
		return fmt.Errorf("square(%d,%d): expected record", col, row)
	}
	tagExpr, ok := rec["tag"]
	if !ok {
		return fmt.Errorf("square(%d,%d): missing tag", col, row)
	}
	tag, ok := tagExpr.Value.(string)
	if !ok {
		return fmt.Errorf("square(%d,%d): tag not string", col, row)
	}

	implSquare := d.game.Cell(col, row)

	switch tag {
	case "Empty":
		if implSquare.occupied {
			return fmt.Errorf("square(%d,%d): spec=Empty impl=Occupied(%v)", col, row, implSquare.player)
		}
	case "Occupied":
		if !implSquare.occupied {
			return fmt.Errorf("square(%d,%d): spec=Occupied impl=Empty", col, row)
		}
		valueExpr, ok := rec["value"]
		if !ok {
			return fmt.Errorf("square(%d,%d): Occupied missing value", col, row)
		}
		sp, err := playerFromExpr(valueExpr)
		if err != nil {
			return fmt.Errorf("square(%d,%d): %w", col, row, err)
		}
		if specPlayer(implSquare.player) != sp {
			return fmt.Errorf("square(%d,%d): player mismatch impl=%v spec=%v",
				col, row, implSquare.player, sp)
		}
	default:
		return fmt.Errorf("square(%d,%d): unknown tag %q", col, row, tag)
	}
	return nil
}

// positionFromNondet tries to extract a (col, row) position from the named
// nondet picks.  primary is tried first, then secondary (if non-empty).
// Returns an error only if the primary is required and missing.
func positionFromNondet(step *connector.Step, primary, secondary string) (Position, error) {
	if e, ok := step.NondetPicks.Get(primary); ok {
		return exprToPosition(e)
	}
	if secondary != "" {
		if e, ok := step.NondetPicks.Get(secondary); ok {
			return exprToPosition(e)
		}
	}
	return Position{}, fmt.Errorf("no position pick found (tried %q, %q)", primary, secondary)
}

// exprToPosition converts an ITF tuple (or list) expression like [col, row] to a Position.
// In Quint a tuple (int,int) is represented as {"#tup": [col, row]} which itf-go parses
// as a ListExprType.
func exprToPosition(e itf.Expr) (Position, error) {
	list, ok := e.Value.(itf.ListExprType)
	if !ok {
		return Position{}, fmt.Errorf("position: expected tuple/list, got %T", e.Value)
	}
	if len(list) != 2 {
		return Position{}, fmt.Errorf("position: expected 2 elements, got %d", len(list))
	}
	col, err := toInt(list[0])
	if err != nil {
		return Position{}, fmt.Errorf("position col: %w", err)
	}
	row, err := toInt(list[1])
	if err != nil {
		return Position{}, fmt.Errorf("position row: %w", err)
	}
	return Position{Col: col, Row: row}, nil
}

func toInt(e itf.Expr) (int, error) {
	switch v := e.Value.(type) {
	case float64:
		return int(v), nil
	case int64:
		return int(v), nil
	}
	return 0, fmt.Errorf("expected number, got %T", e.Value)
}

// ---- Test ----

// TestTicTacToe runs the tictactoe simulation using quint-connector-go.
// This is a Go port of the #[quint_run] test in mbt.rs.
// The test is skipped if the quint CLI is not installed.
func TestTicTacToe(t *testing.T) {
	connector.SkipIfNoQuint(t)

	driver := &TicTacToeDriver{}
	connector.RunSimulation(t, driver, connector.RunConfig{
		Spec:       "spec/tictactoe.qnt",
		MaxSamples: 1,
	})
}
