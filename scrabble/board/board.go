package board

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	spacesRegexp *regexp.Regexp
)

func init() {
	spacesRegexp, _ = regexp.Compile(`\s+`)
}

type Board struct {
	config []byte
}

// Create a new empty scrabble board, with 15x15 spaces and no constraints.
func New() (board Board) {
	board.config = make([]byte, 15*15)
	for i := range(board.config) {
		board.config[i] = byte('.')
	}
	return
}

// Create a new scrabble board from a string.
//
// The string should be in row-major order, left-to-right, top-to-bottom.
// All whitespace is stripped, and the string is expected to contain 15*15
// non-space values, otherwise a panic ensues.
// The string is normalized to be all upper-case. Also, the '.' character means
// "unconstrained".
func NewFromString(config string) (board Board) {
	normalized := spacesRegexp.ReplaceAllLiteralString(strings.ToUpper(config), "")
	if len(normalized) != 15*15 {
		panic(fmt.Sprintf("Scrabble board not 15x15: %v", normalized))
	}
	board.config = []byte(normalized)
	return
}

// Output a simple string representation of the board, row-major order.
func (board Board) String() string {
	s := make([]string, 0, 15)
	for config := board.config; len(config) > 0; config = config[15:] {
		s = append(s, string(config[:15]))
	}
	return strings.Join(s, "\n")
}

// Generate constraint queries (row, col) for this position.
//
// For the row-wise constraint, the column is checked for adjacent letters if
// the position given is otherwise unconstrained. For col-wise, the *row* is
// checked (so that row-wise means "we're trying to lay down a row" and
// col-wise means "we're trying to lay down a column).
func (board Board) PositionQueries(row, col int) (rowq, colq string) {
	atPos := board.config[row*15 + col]
	rowq = string(atPos)
	colq = string(atPos)
	if atPos == '.' {
		// Search the column for adjacent constraints.
		r0 := row - 1
		r1 := row + 1
		for ; r0 >= 0; r0-- {
			if board.config[r0*15 + col] == '.' {
				break
			}
		}
		r0++
		for ; r1 < 15; r1++ {
			if board.config[r1*15 + col] == '.' {
				break
			}
		}
		// Search the row for adjacent constraints.
		c0 := col - 1
		c1 := col + 1
		for ; c0 >= 0; c0-- {
			if board.config[row*15 + c0] == '.' {
				break
			}
		}
		c0++
		for ; c1 < 15; c1++ {
			if board.config[row*15 + c1] == '.' {
				break
			}
		}
		// If we have constraints, change to a different form of query with <>.
		// We may have a row constraint of [r0, r1) or a column constraint of [c0, c1).
		if r0 != row || r1 != row + 1 {
			constraints := make([]string, r1 - r0)
			for i := 0; i < r1 - r0; i++ {
				constraints[i] = string(board.config[(r0 + i)*15 + col])
			}
			rowq = fmt.Sprintf("<%s>", strings.Join(constraints, ""))
		}
		if c0 != col || c1 != col + 1 {
			colq = fmt.Sprintf("<%s>", string(board.config[row*15+c0 : row*15+c1]))
		}
	}
	return
}

// Return a row query for the given row. Includes constraints from adjacent
// letters if there is a blank somewhere on the row.
func (board Board) RowQuery(row int) string {
	pieces := make([]string, 15)
	for col := 0; col < 15; col++ {
		pieces[col], _ = board.PositionQueries(row, col)
	}
	return strings.Join(pieces, "")
}

// Return a column query for the given column, similar to RowQuery.
func (board Board) ColQuery(col int) string {
	pieces := make([]string, 15)
	for row := 0; row < 15; row++ {
		_, pieces[row] = board.PositionQueries(row, col)
	}
	return strings.Join(pieces, "")
}
