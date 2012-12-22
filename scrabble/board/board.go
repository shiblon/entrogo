package board

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	spacesRegexp      *regexp.Regexp
	wordMultipliers   = map[string]int{"*": 3, "+": 2}
	letterMultipliers = map[string]int{"$": 3, "#": 2}
	specials          = []string{
		"...*..$.$..*...",
		"..#..+...+..#..",
		".#..#.....#..#.",
		"*..$...+...$..*",
		"..#...#.#...#..",
		".+...$...$...+.",
		"$...#.....#...$",
		"...+.......+...",
		"$...#.....#...$",
		".+...$...$...+.",
		"..#...#.#...#..",
		"*..$...+...$..*",
		".#..#.....#..#.",
		"..#..+...+..#..",
		"...*..$.$..*...",
	}
	scores = map[rune]int{
		'A': 1, 'B': 3, 'C': 3, 'D': 2, 'E': 1, 'F': 4, 'G': 2, 'H': 4, 'I': 1,
		'J': 8, 'K': 5, 'L': 1, 'M': 3, 'N': 2, 'O': 1, 'P': 3, 'Q': 10, 'R': 1,
		'S': 1, 'T': 1, 'U': 1, 'V': 4, 'W': 4, 'X': 8, 'Y': 4, 'Z': 10,
	}
)

func init() {
	spacesRegexp, _ = regexp.Compile(`\s+`)
}

type Board struct {
	config []byte
}

type PositionInfo struct {
	Row, Col           int

	// Constraint query if placing along a row or column,
	// respectively.
	RowQuery, ColQuery string

	// Score (sum) of adjacent tiles if placing a tile along a row
	// or column. This means that if we place a tile here (along a
	// row), we will connect up a word where the sum of existing
	// tiles (not including the one we placed) is RowScore. This
	// can be used to compute intersecting word scores, etc.
	RowScore, ColScore int
}

// Create a new empty scrabble board, with 15x15 spaces and no constraints.
func New() (board Board) {
	board.config = make([]byte, 15*15)
	for i := range board.config {
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

func (board Board) BlankAt(row, col int) bool {
	// TODO: implement a way to specify blank tiles.
	return false
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
func (board Board) PositionInfo(row, col int) PositionInfo {
	atPos := board.config[row*15+col]
	info := PositionInfo{
		Row:      row,
		Col:      col,
		RowScore: 0,
		ColScore: 0,
		RowQuery: string(atPos),
		ColQuery: string(atPos),
	}
	if atPos == '.' {
		// Search the column for adjacent constraints.
		r0 := row - 1
		r1 := row + 1
		for ; r0 >= 0; r0-- {
			if board.config[r0*15+col] == '.' {
				break
			}
		}
		r0++
		for ; r1 < 15; r1++ {
			if board.config[r1*15+col] == '.' {
				break
			}
		}
		// Search the row for adjacent constraints.
		c0 := col - 1
		c1 := col + 1
		for ; c0 >= 0; c0-- {
			if board.config[row*15+c0] == '.' {
				break
			}
		}
		c0++
		for ; c1 < 15; c1++ {
			if board.config[row*15+c1] == '.' {
				break
			}
		}
		// If we have constraints, include all letters that must be near it, e.g., "QU.RK".
		// We may have a row constraint of [r0, r1) or a column constraint of [c0, c1).
		if r0 != row || r1 != row+1 {
			constraints := make([]string, r1-r0)
			for i := 0; i < r1-r0; i++ {
				constraints[i] = string(board.config[(r0+i)*15+col])
			}
			info.RowQuery = strings.Join(constraints, "")
			for i, c := range info.RowQuery {
				if c != '.' && !board.BlankAt(r0 + i, col) {
					info.RowScore += scores[c]
				}
			}
		}
		if c0 != col || c1 != col+1 {
			info.ColQuery = string(board.config[row*15+c0:row*15+c1])
			for i, c := range info.ColQuery {
				if c != '.' && !board.BlankAt(row, col + i) {
					info.ColScore += scores[c]
				}
			}
		}
	}
	return info
}

// Return a row query for the given row. Includes constraints from adjacent
// letters if there is a blank somewhere on the row.
func (board Board) RowQuery(row int) string {
	pieces := make([]string, 15)
	for col := 0; col < 15; col++ {
		q := board.PositionInfo(row, col).RowQuery
		if len([]rune(q)) > 1 {
			q = "<" + q + ">"
		}
		pieces[col] = q
	}
	return strings.Join(pieces, "")
}

// Return a column query for the given column, similar to RowQuery.
func (board Board) ColQuery(col int) string {
	pieces := make([]string, 15)
	for row := 0; row < 15; row++ {
		q := board.PositionInfo(row, col).ColQuery
		if len([]rune(q)) > 1 {
			q = "<" + q + ">"
		}
		pieces[row] = q
	}
	return strings.Join(pieces, "")
}

// TODO: Write a function to score a valid placement of a set of
// tiles (including word and letter multipliers, and the ability
// to understand blanks).
