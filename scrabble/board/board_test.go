package board

import (
	"testing"
)

const testString = ("...............\n" +
	"...... ..... ....  " +
	"..............." +
	"..............." +
	"..............." +
	"..............." +
	"..............." +
	"..abc.RT.uxvs.." +
	"....abclsd....." +
	"..............." +
	"..............." +
	"..............." +
	"..............." +
	"..............." +
	"...............")

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

func TestPositionQueries(t *testing.T) {
	board := NewFromString(testString)

	testf := func(row, col int, rquery, cquery string) {
		info := board.PositionInfo(row, col)
		if rq, cq := info.RowQuery, info.ColQuery; rq != rquery && cq != cquery {
			t.Errorf("Unexpected value at %v,%v: %v, %v", row, col, rq, cq)
		}
	}

	testf(6, 1, ".", ".")
	testf(6, 2, ".A", ".")
	testf(6, 4, ".CA", ".")
	testf(7, 5, ".B", "ABC.RT")
	testf(7, 10, "X", "X")
}

func TestRowQueries(t *testing.T) {
	board := NewFromString(testString)

	testf := func(row int, query string) {
		if rq := board.RowQuery(row); rq != query {
			t.Errorf("Unexpected row query at row %v: %v", row, rq)
		}
	}

	testf(5, "...............")
	testf(6, "..<.A><.B><.CA>.<.RC><.TL>.<.UD><.X><.V><.S>..")
	testf(7, "..ABC<.B>RT<.S>UXVS..")
	testf(8, "..<A.><B.>ABCLSD<X.><V.><S.>..")
	testf(9, "....<CA.><B.><RC.><TL.><S.><UD.>.....")
	testf(10, "...............")
}

func TestColQueries(t *testing.T) {
	board := NewFromString(testString)

	testf := func(col int, query string) {
		if cq := board.ColQuery(col); cq != query {
			t.Errorf("Unexpected col query at col %v: %v", col, cq)
		}
	}

	testf(0, "...............")
	testf(1, ".......<.ABC>.......")
	testf(2, ".......A.......")
	testf(3, ".......B<.ABCLSD>......")
	testf(14, "...............")
}
