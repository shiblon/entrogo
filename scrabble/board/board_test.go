package board

import (
	"fmt"
	"testing"
)

const testString = "" +
	"...............\n" +
	"...... ..... ....  " +
	"..............." +
	"..............." +
	"..............." +
	"..............." +
	"...............\n\n" +
	"..abc.RT.uxvs.." +
	"....abclsd....." +
	"..............." +
	"..............." +
	"..............." +
	"..............." +
	"..............." +
	"..............."

func TestNew(t *testing.T) {
	board := New()
	if len(board.config) != 15*15 {
		t.Error("New board is not 15x15")
	}
	for _, v := range board.config {
		if v != '.' {
			t.Errorf("Unexpected unconstrained character '%v' in new board", v)
		}
	}
}

func TestNewFromString(t *testing.T) {
	board := NewFromString(testString)

	if len(board.config) != 15*15 {
		t.Error("New board is not 15x15")
	}

	if board.config[107] != 'A' {
		t.Errorf("Unexpected character '%v'", string(board.config[107]))
	}

	if board.config[117] != 'S' {
		t.Errorf("Unexpected character '%v'", string(board.config[117]))
	}
}

func TestPositionInfo(t *testing.T) {
	board := NewFromString(testString)

	testf := func(info PositionInfo) {
		obtained := board.PositionInfo(info.Row, info.Col)
		if info.RowQuery != obtained.RowQuery || info.ColQuery != obtained.ColQuery ||
			info.RowScore != obtained.RowScore || info.ColScore != obtained.ColScore ||
			info.Row != obtained.Row || info.Col != obtained.Col ||
			info.PosScore != obtained.PosScore {
			t.Errorf("Unexpected position info. Wanted\n%v\nGot\n%v", info, obtained)
		}
	}

	testf(PositionInfo{6, 1, ".", ".", 0, 0, 0})
	testf(PositionInfo{6, 2, ".A", ".", 0, 1, 0})
	testf(PositionInfo{6, 4, ".CA", ".", 0, 4, 0})
	testf(PositionInfo{7, 5, ".B", "ABC.RT", 0, 3, 9})
	testf(PositionInfo{7, 10, "X", "X", 8, 0, 0})
}

func strSliceEq(a, b []string) bool {
	return fmt.Sprintf("%#v", a) == fmt.Sprintf("%#v", b)
}

func TestRowQueries(t *testing.T) {
	board := NewFromString(testString)

	testf := func(row int, query []string) {
		if rq := board.RowQuery(row); !strSliceEq(rq, query) {
			t.Errorf("Unexpected row query at row %v: %v", row, rq)
		}
	}

	testf(5, []string{".", ".", ".", ".", ".", ".", ".", ".", ".", ".", ".", ".", ".", ".", "."})
	testf(6, []string{".", ".", ".A", ".B", ".CA", ".", ".RC", ".TL", ".", ".UD", ".X", ".V", ".S", ".", "."})
	testf(7, []string{".", ".", "A", "B", "C", ".B", "R", "T", ".S", "U", "X", "V", "S", ".", "."})
	testf(8, []string{".", ".", "A.", "B.", "A", "B", "C", "L", "S", "D", "X.", "V.", "S.", ".", "."})
	testf(9, []string{".", ".", ".", ".", "CA.", "B.", "RC.", "TL.", "S.", "UD.", ".", ".", ".", ".", "."})
	testf(10, []string{".", ".", ".", ".", ".", ".", ".", ".", ".", ".", ".", ".", ".", ".", "."})
}

func TestColQueries(t *testing.T) {
	board := NewFromString(testString)

	testf := func(col int, query []string) {
		if cq := board.ColQuery(col); !strSliceEq(cq, query) {
			t.Errorf("Unexpected col query at col %v: %v", col, cq)
		}
	}

	testf(0, []string{".", ".", ".", ".", ".", ".", ".", ".", ".", ".", ".", ".", ".", ".", "."})
	testf(1, []string{".", ".", ".", ".", ".", ".", ".", ".ABC", ".", ".", ".", ".", ".", ".", "."})
	testf(2, []string{".", ".", ".", ".", ".", ".", ".", "A", ".", ".", ".", ".", ".", ".", "."})
	testf(3, []string{".", ".", ".", ".", ".", ".", ".", "B", ".ABCLSD", ".", ".", ".", ".", ".", "."})
	testf(14, []string{".", ".", ".", ".", ".", ".", ".", ".", ".", ".", ".", ".", ".", ".", "."})
}

func TestScorePlacement(t *testing.T) {
	board := NewFromString(testString)

	testf := func(row, col int, placement string, score int) {
		if s := board.ScoreColPlacement(col, row, placement); s != score {
			t.Errorf("Score for col placement %v,%v,%v should be %v, was %v",
				row, col, placement, score, s)
		}
	}

	testf(4, 4, "BIGCAT", 13)          // check that 'placing' existing tiles works
	testf(4, 4, "BIG  T", 13)          // check that ' ' means 'do not place'
	testf(4, 4, "Big  T", 13)          // check multiple cases
	testf(7, 7, "Tlinglit", 20)        // double word score
	testf(6, 7, "atlinglit", 22+bingo) // double word score, used 7 letters
	testf(3, 8, "bricks", 36)          // k intersects with an existing word
}

// TODO: Test the scoring functions.
