package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"monson/scrabble/board"
	"monson/scrabble/index"
	"os"
	"sort"
	"strings"
)

const (
	RIGHT = iota
	DOWN
)

var (
	recognizerFile = flag.String("recognizer", "wordswithfriends.mealy",
		"Serialized Mealy machine to use as a word recognizer.")
)

// TODO:
// Use a full board instead of just a single line.
// Remove this whole var section and all scoring functions - favor the board functions instead.
var (
	LetterScores = map[rune]int{
		'A': 1, 'B': 3, 'C': 3, 'D': 2,
		'E': 1, 'F': 4, 'G': 2, 'H': 4,
		'I': 1, 'J': 8, 'K': 5, 'L': 1,
		'M': 3, 'N': 2, 'O': 1, 'P': 3,
		'Q': 10, 'R': 1, 'S': 1, 'T': 1,
		'U': 1, 'V': 4, 'W': 4, 'X': 8,
		'Y': 4, 'Z': 10,
	}

	LetterMultipliers = map[string]int{"$": 3, "#": 2}

	WordMultipliers = map[string]int{"*": 3, "+": 2}

	Board = []string{
		"...*..$.$..*...",
		"..#..+...+..#..",
		".#..#.....#..#.",
		"*..$...+...$..*",
		"..#...#.#...#..",
		".+...$...$...+.",
		"$...#.....#...$",
		"...+.......+...",
	}
)

// An imperfect score for a sequence. If line is 0, just score the letters,
// otherwise try to figure it out based on the board (1-based line count).
// TODO: Add in information about intersecting words.
// TODO: Don't score fixed letters (they don't get extra scores).

// line and pos are both *1-based*.
func SeqScore(seq []byte, line, pos int, draws []bool) int {
	if len(draws) == 0 {
		draws = make([]bool, len(seq))
		for i := 0; i < len(draws); i++ {
			draws[i] = true
		}
	}
	s := 0
	wordMultiplier := 1
	for i, v := range seq {
		p := pos + i
		ls := LetterScores[rune(v)]
		if line > 0 {
			b := string(Board[line-1][p-1])
			if draws[i] { // multipliers only apply to placed tiles
				wm := WordMultipliers[b]
				lm := LetterMultipliers[b]
				if wm > 0 {
					wordMultiplier *= wm
				}
				if lm > 0 {
					ls *= lm
				}
			}
		}
		s += ls
	}
	s *= wordMultiplier
	return s
}

func GetSubSuffixes(info index.AllowedInfo) <-chan int {
	out := make(chan int)
	emit := func(left int) {
		if info.AnchoredSubSequence(left, len(info.Draws)) {
			out <- left
		}
	}
	go func() {
		defer close(out)
		for left := 0; left < len(info.Constraints); left++ {
			emit(left)
			for left < len(info.Constraints) && !info.Draws[left] {
				left++
			}
		}
	}()
	return out
}

type Endpoints struct {
	left  int
	right int
}

func GetSubConstraints(info index.AllowedInfo) <-chan Endpoints {
	out := make(chan Endpoints)
	emit := func(left, right int) {
		if info.AnchoredSubSequence(left, right) {
			out <- Endpoints{left, right}
		}
	}
	go func() {
		defer close(out)
		for left := range GetSubSuffixes(info) {
			emit(left, len(info.Constraints))
			for right := left + 1; right < len(info.Constraints); right++ {
				// We can't peel off a fixed tile - it must form part of the
				// word.
				if !info.Draws[right] {
					continue
				}
				emit(left, right)
			}
		}
	}()
	return out
}

func FormatSeq(word []byte, left, origSize int) []byte {
	pieces := make([]byte, origSize)
	for i := 0; i < left; i++ {
		pieces[i] = '_'
	}
	for i, c := range word {
		pieces[i+left] = c
	}
	for i := left + len(word); i < origSize; i++ {
		pieces[i] = '_'
	}
	return pieces
}

func TileCounts(available string) (counts map[byte]int) {
	counts = make(map[byte]int)
	for _, c := range strings.ToUpper(available) {
		counts[byte(c)]++
	}
	return
}

type foundword struct {
	word      string
	line      int
	direction int
	start     int
	score     int
}

func (self foundword) String() string {
	switch self.direction {
	case RIGHT:
		return fmt.Sprintf("%s (%d): row %d, start %d", self.word, self.score, self.line, self.start)
	case DOWN:
		return fmt.Sprintf("%s (%d): col %d, start %d", self.word, self.score, self.line, self.start)
	}
	return fmt.Sprintf("%s (%d): direction? line %d, start %d",
		self.word, self.score, self.line, self.start)
}

// Make it sortable
type foundwords []foundword

func (self foundwords) Len() int {
	return len(self)
}
func (self foundwords) Less(a, b int) bool {
	return self[a].score < self[b].score
}
func (self foundwords) Swap(a, b int) {
	self[a], self[b] = self[b], self[a]
}

func LineWords(line, direction int, idx index.Index, lineQuery []string, available map[byte]int) <-chan foundword {
	allowed := idx.GetAllowedLetters(lineQuery, available)

	out := make(chan foundword)
	go func() {
		defer close(out)
		for left := range GetSubSuffixes(allowed) {
			subinfo := allowed.MakeSuffix(left)
			if !subinfo.PossiblePrefix() {
				continue
			}
			for seq := range idx.ConstrainedSequences(subinfo) {
				out <- foundword{
					word:      string(seq),
					start:     left,
					line:      line,
					direction: direction,
				}
			}
		}
	}()
	return out
}

func InitialWords(idx index.Index, available map[byte]int) <-chan foundword {
	allowed := index.NewUnanchoredAllowedInfo(
		[]string{".", ".", ".", ".", ".", ".", "."},
		[]bool{true, true, true, true, true, true, true},
		available)

	out := make(chan foundword)

	go func() {
		defer close(out)
		for left := 0; left < 7; left++ {
			subinfo := allowed.MakeSuffix(left)
			for seq := range idx.ConstrainedSequences(subinfo) {
				out <- foundword{
					word: string(seq),
					start: 7,
					line: 7,
					direction: RIGHT,
				}
			}
		}
	}()

	return out
}

func BoardWords(board board.Board, idx index.Index, available map[byte]int) <-chan foundword {
	out := make(chan foundword)

	go func() {
		defer close(out)

		if board.IsEmpty() {
			for found := range InitialWords(idx, available) {
				found.score = board.ScoreRowPlacement(7, found.start, found.word)
				out <- found
			}
			return
		}

		for row := 0; row < 15; row++ {
			q := board.RowQuery(row)
			for found := range LineWords(row, RIGHT, idx, q, available) {
				found.score = board.ScoreRowPlacement(row, found.start, found.word)
				out <- found
			}
		}

		for col := 0; col < 15; col++ {
			q := board.ColQuery(col)
			for found := range LineWords(col, DOWN, idx, q, available) {
				found.score = board.ScoreColPlacement(col, found.start, found.word)
				out <- found
			}
		}
	}()

	return out
}

func main() {
	flag.Parse()

	// Read the recognizer.
	fmt.Print("Raeding recognizer...")
	rfile, err := os.Open(*recognizerFile)
	if err != nil {
		log.Fatalf("Failed to open '%v': %v", *recognizerFile, err)
	}
	idx, err := index.ReadFrom(rfile)
	if err != nil {
		log.Fatalf("Failed to read recognizer from '%v': %v", *recognizerFile, err)
	}
	fmt.Println("DONE")

	// Read the board.
	boardbytes, err := ioutil.ReadFile(flag.Arg(0))
	if err != nil {
		log.Fatalf("Could not load board file '%v': %v", flag.Arg(0), err)
	}
	board := board.NewFromString(string(boardbytes))
	available := TileCounts(flag.Arg(1))

	// Show what board and tiles we are working with.
	fmt.Println(board)
	for k, v := range available {
		fmt.Printf("%v: %v   ", string(k), v)
	}
	fmt.Println()

	// Get and score all words:
	allwords := make([]foundword, 0, 500)
	for word := range BoardWords(board, idx, available) {
		allwords = append(allwords, word)
	}

	// Sort by score, descending.
	sort.Sort(foundwords(allwords))

	for _, w := range allwords {
		fmt.Println(w)
	}
}
