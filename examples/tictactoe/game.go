// Package tictactoe implements a tic-tac-toe game for model-based testing.
//
// This is a Go port of the tictactoe example from quint-connect (Rust).
// It demonstrates how to use quint-connector-go with a simple, well-understood
// game to verify a Go implementation against a Quint specification.
package tictactoe

// Player represents a player in tic-tac-toe (X or O).
type Player int

const (
	PlayerX Player = iota
	PlayerO
)

func (p Player) String() string {
	if p == PlayerX {
		return "X"
	}
	return "O"
}

// Square is a cell on the board: either empty or occupied by a player.
type Square struct {
	occupied bool
	player   Player
}

// GameBoard is a 3×3 tic-tac-toe board indexed [col][row] (1-based).
type GameBoard [3][3]Square

// TicTacToe holds the mutable game state.
type TicTacToe struct {
	Board    GameBoard
	NextTurn Player
}

// Position is a (col, row) coordinate (1-based, matching the Quint spec).
type Position struct {
	Col, Row int
}

// MoveTo places the given player's mark at pos.
// Panics (like the Rust assert!) if the cell is occupied or it is not this player's turn.
func (g *TicTacToe) MoveTo(pos Position, player Player) {
	if g.NextTurn != player {
		panic("player out of turn")
	}
	c, r := pos.Col-1, pos.Row-1
	if g.Board[c][r].occupied {
		panic("moving to occupied cell")
	}
	g.Board[c][r] = Square{occupied: true, player: player}
	if player == PlayerX {
		g.NextTurn = PlayerO
	} else {
		g.NextTurn = PlayerX
	}
}

// Cell returns the square at (col, row) 1-based.
func (g *TicTacToe) Cell(col, row int) Square {
	return g.Board[col-1][row-1]
}
