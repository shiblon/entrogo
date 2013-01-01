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

// Flags
var (
	recognizerFile = flag.String("recognizer", "wordswithfriends.mealy",
		"Serialized Mealy machine to use as a word recognizer.")
)

// Directions
const (
	RIGHT = iota
	DOWN
)

// Produce all valid "left" indices for a particular search query.
// What determines whether an index is valid is whether it can form the start
// of a complete word. This basically means "any cell with nothing on the
// left".
//
// A cell that has *something* on the left of it cannot form the beginning of a
// word because it must have a prefix (the stuff on the left) to be a word.
//
// Given a query like  ..AD.., this will produce the indices 0, 1, 2, 5
// (because indices 3 and 4 are 'D' and the '.' after it, which can't be the
// beginning of a word, but index 2 can, since a word can start with 'AD' in
// this scenario).
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

// Endpoints, left inclusive, right exclusive: [left, right)
type Endpoints struct {
	left  int
	right int
}

// Get all allowed subconstraints from an initial constraint.
//
// This constitutes peeling off from the left and right whatever can be peeled
// off to form a new sub constraint.
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

// Create a map from byte to count, given a string containing all tiles
// (including '.' for blank tiles).
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
		return fmt.Sprintf("%s (%d): %d, %d across", self.word, self.score, self.line, self.start)
	case DOWN:
		return fmt.Sprintf("%s (%d): %d, %d down", self.word, self.score, self.start, self.line)
	}
	return fmt.Sprintf("%s (%d): direction? line %d, start %d",
		self.word, self.score, self.line, self.start)
}

type foundwords []foundword

func (self foundwords) Len() int {
	return len(self)
}
func (self foundwords) Less(a, b int) bool {
	// Sort by score.
	return self[a].score < self[b].score
}
func (self foundwords) Swap(a, b int) {
	self[a], self[b] = self[b], self[a]
}

// Given a line-oriented query (find me a word that matches this sort of line),
// produce all valid words on that line.
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

// Given a blank board, find all words we can play with our given tiles, from the center.
//
// Since the center spot is always required, we just always start our words
// from there. No word will ever be long enough to go off the edge of the
// board, since that is 8 tiles away and we only ever have 7.
//
// Also, direction is not important because the board has 4-way symmetry. So,
// we always choose 7,7,RIGHT.
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
					word:      string(seq),
					start:     7,
					line:      7,
					direction: RIGHT,
				}
			}
		}
	}()

	return out
}

// Get all words that can be formed on this board with the available tiles, and
// score all of them.
//
// Empty boards are also allowed, in which case all words formable with the
// available tiles are used, starting at 7,7 and going to the right.
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

	// Sort by score, ascending.
	sort.Sort(foundwords(allwords))

	for _, w := range allwords {
		fmt.Println(w)
	}
}
