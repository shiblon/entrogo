package board

// TODO:
// Add case-sensitivity to allow blank tiles to be specified.

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	spacesRegexp      *regexp.Regexp
	wordMultipliers   = map[rune]int{'*': 3, '+': 2}
	letterMultipliers = map[rune]int{'$': 3, '#': 2}
	bingo             = 35 // points for bingo
	/*
		// Scrabble
		specials          = [][]rune{
			[]rune("*..#...*...#..*"),
			[]rune(".+...$...$...+."),
			[]rune("..+...#.#...+.."),
			[]rune("#..+...#...+..#"),
			[]rune("....+.....+...."),
			[]rune(".$...$...$...$."),
			[]rune("..#...#.#...#.."),
			[]rune("*..#...+...#..*"),
			[]rune("..#...#.#...#.."),
			[]rune(".$...$...$...$."),
			[]rune("....+.....+...."),
			[]rune("#..+...#...+..#"),
			[]rune("..+...#.#...+.."),
			[]rune(".+...$...$...+."),
			[]rune("*..#...*...#..*"),
		}
		scores = map[rune]int{
			'A': 1, 'B': 3, 'C': 3, 'D': 2, 'E': 1, 'F': 4, 'G': 2, 'H': 4, 'I': 1,
			'J': 8, 'K': 5, 'L': 1, 'M': 3, 'N': 1, 'O': 1, 'P': 3, 'Q': 10, 'R': 1,
			'S': 1, 'T': 1, 'U': 1, 'V': 4, 'W': 4, 'X': 8, 'Y': 4, 'Z': 10,
		}
	*/
	// Words with Friends
	specials = [][]rune{
		[]rune("...*..$.$..*..."),
		[]rune("..#..+...+..#.."),
		[]rune(".#..#.....#..#."),
		[]rune("*..$...+...$..*"),
		[]rune("..#...#.#...#.."),
		[]rune(".+...$...$...+."),
		[]rune("$...#.....#...$"),
		[]rune("...+.......+..."),
		[]rune("$...#.....#...$"),
		[]rune(".+...$...$...+."),
		[]rune("..#...#.#...#.."),
		[]rune("*..$...+...$..*"),
		[]rune(".#..#.....#..#."),
		[]rune("..#..+...+..#.."),
		[]rune("...*..$.$..*..."),
	}
	scores = map[rune]int{
		'A': 1, 'B': 4, 'C': 4, 'D': 2, 'E': 1, 'F': 4, 'G': 3, 'H': 3, 'I': 1,
		'J': 10, 'K': 5, 'L': 2, 'M': 4, 'N': 2, 'O': 1, 'P': 4, 'Q': 10, 'R': 1,
		'S': 1, 'T': 1, 'U': 2, 'V': 5, 'W': 4, 'X': 8, 'Y': 3, 'Z': 10,
	}
)

func init() {
	spacesRegexp, _ = regexp.Compile(`\s+`)
}

func MultipliersAt(row, col int) (letter, word int) {
	special := specials[row][col]
	letter = 1
	word = 1
	if lm, ok := letterMultipliers[special]; ok {
		letter = lm
	}
	if wm, ok := wordMultipliers[special]; ok {
		word = wm
	}
	return
}

type PositionInfo struct {
	Row, Col int

	// Constraint query if placing along a row or column,
	// respectively.
	RowQuery, ColQuery string

	// Score (sum) of adjacent tiles if placing a tile along a row
	// or column. This means that if we place a tile here (along a
	// row), we will connect up a word where the sum of existing
	// tiles (not including the one we placed) is RowScore. This
	// can be used to compute intersecting word scores, etc.
	// The position score is the score of the letter at this position, if there
	// is one.
	PosScore, RowScore, ColScore int
}

type Board struct {
	config []rune
	// Cached info lists.
	positionInfoCache [15 * 15]*PositionInfo
}

// Create a new empty scrabble board, with 15x15 spaces and no constraints.
func New() (board Board) {
	board.config = make([]rune, 15*15)
	for i := range board.config {
		board.config[i] = '.'
	}
	return
}

// Return true if there are no tiles placed on this board.
func (board Board) IsEmpty() (empty bool) {
	empty = true
	for _, val := range board.config {
		if val != '.' {
			empty = false
		}
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
	runes := []rune(normalized)
	if len(runes) != 15*15 {
		panic(fmt.Sprintf("Scrabble board not 15x15: %v", normalized))
	}
	board.config = runes
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

	var info *PositionInfo
	if idx := row*15 + col; board.positionInfoCache[idx] != nil {
		return *board.positionInfoCache[idx]
	}
	info = &PositionInfo{
		Row:      row,
		Col:      col,
		RowQuery: string(atPos),
		ColQuery: string(atPos),
		PosScore: scores[atPos],
		RowScore: 0,
		ColScore: 0,
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
				if c != '.' && !board.BlankAt(r0+i, col) {
					info.RowScore += scores[c]
				}
			}
		}
		if c0 != col || c1 != col+1 {
			info.ColQuery = string(board.config[row*15+c0 : row*15+c1])
			for i, c := range info.ColQuery {
				if c != '.' && !board.BlankAt(row, col+i) {
					info.ColScore += scores[c]
				}
			}
		}
	}
	board.positionInfoCache[row*15+col] = info
	return *info
}

// Return a row query for the given row. Includes constraints from adjacent
// letters if there is a blank somewhere on the row.
func (board Board) RowQuery(row int) []string {
	pieces := make([]string, 15)
	for col := 0; col < 15; col++ {
		pieces[col] = board.PositionInfo(row, col).RowQuery
	}
	return pieces
}

// Return a column query for the given column, similar to RowQuery.
func (board Board) ColQuery(col int) []string {
	pieces := make([]string, 15)
	for row := 0; row < 15; row++ {
		pieces[row] = board.PositionInfo(row, col).ColQuery
	}
	return pieces
}

// Score a particular placement. This is generic and works with either rows or
// columns, which is why the interface is somewhat weird.
//
// Args:
// 	places: a slice of runes with the placement info. '_' is a blank tile, ' ' means "do nothing".
// 	getStuff: a function that returns important information for the calculation:
// 		adjq: the adjacency query (e.g., RowQuery from PositionInfo)
// 		posscore: the score of *this letter* if it's already on the board (not being placed)
// 		adjscore: the score of the letters in the adjacency query
// 		lmul: the letter multiplier at this position
// 		wmul: the word multiplier at this position
// 		blank: whether this position is a blank tile already on the board (not being placed)
//
// Returns an integer word placement score (including multipliers and bingo calculation).
func (board Board) ScorePlacement(
	places []rune,
	getStuff func(i int) (adjq string, posscore, adjscore, lmul, wmul int, blank bool)) int {

	wordMultiplier := 1
	adjacentScore := 0
	thisScore := 0
	numPlaced := 0

	for i, c := range places {
		adjq, posscore, adjscore, lmul, wmul, blank := getStuff(i)

		// If this is a fixed tile (there is no . anywhere), then we just add
		// the score for this tile with nothing else fancy going on.
		if strings.IndexRune(adjq, '.') == -1 {
			if c != ' ' && adjq != string(c) {
				panic(fmt.Sprintf(
					"Incorrect placement '%v' for fixed tile '%v'", string(c), adjq))
			}
			if !blank {
				thisScore += posscore
			}
			continue
		}

		// Not a fixed point, but a placed tile. We compute not only the
		// running score, but we also apply letter multipliers and compute
		// adjacency scores (with word multipliers).

		numPlaced++
		letterScore := 0

		wordMultiplier *= wmul

		if c != '_' {
			letterScore = scores[c] * lmul
		}

		// We already know it *has* a '.', this tests whether there are constraining tiles to score.
		if adjq != "." {
			adjacentScore += (letterScore + adjscore) * wmul
		}
		thisScore += letterScore // word multiplier comes later.
	}

	thisScore *= wordMultiplier
	thisScore += adjacentScore
	if numPlaced == 7 {
		thisScore += bingo
	}
	return thisScore
}

// Score a placement of tiles for the given row, starting at the startColumn.
// A _ means "blank tile" (and therefore won't get scored).
// Fixed values can be specified as " " or as the actual fixed value.
func (board Board) ScoreRowPlacement(row, startCol int, placement string) int {
	return board.ScorePlacement(
		[]rune(strings.ToUpper(placement)),
		func(i int) (string, int, int, int, int, bool) {
			info := board.PositionInfo(row, startCol+i)
			lm, wm := MultipliersAt(row, startCol+i)
			blank := board.BlankAt(row, startCol+i)
			return info.RowQuery, info.PosScore, info.RowScore, lm, wm, blank
		})
}

// Score a placement of tiles for the given column, starting at the startRow.
// A _ means "blank tile" (and therefore won't get scored).
// Fixed values can be specified as " " or as the actual fixed value.
func (board Board) ScoreColPlacement(col, startRow int, placement string) int {
	return board.ScorePlacement(
		[]rune(strings.ToUpper(placement)),
		func(i int) (string, int, int, int, int, bool) {
			info := board.PositionInfo(startRow+i, col)
			lm, wm := MultipliersAt(startRow+i, col)
			blank := board.BlankAt(startRow+i, col)
			return info.ColQuery, info.PosScore, info.ColScore, lm, wm, blank
		})
}
