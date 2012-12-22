package main

import (
	"flag"
	"fmt"
	"log"
	"monson/mealy"
	"monson/scrabble/index"
	"os"
	"regexp"
	"strings"
)

var (
	Line = flag.Int("line", 0, "Row or column for this request - helps with scoring - 0 to skip.")
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

// Parse a query string and return a list of constraint strings that can be used
// to find valid words (assuming an unlimited supply of arbitrary letters).
// Constraint strings are just letters or . for a wild. A constraint string
// specifies either a complete match (tile already there), a full wild ('.') or
// a constrained wild (e.g., '.in').
//
// 	All letters are converted to uppercase before proceeding.
//
// 	If the string contains [...], then that's the available letter list
// 	(they can be repeated, and "." means "blank".
//
// 	The rest of the query can be bounded on either or both sides by "|",
// 	meaning that we should only find words that are bounded in that way.
//
// 	Other than bounds markers, the syntax is "." for any letter, "X" (a
// 	letter) for a specific letter that must be there, and <MA.PING> (dot
// 	must be present) for a letter that has to form a legal word in the
// 	given "." spot.
func ParseQuery(query string) (constraints []string) {
	query = strings.ToUpper(query)
	// Query strings are themselves basically describable with a repeated
	// regular expression, where | is only allowed at the beginning or end
	// of the expression:
	pieceExp := `([.|[:alpha:]])|<([[:alpha:]]*?[.][[:alpha:]]*?)>`
	validExp := `^[|]?(([.[:alpha:]])|(<[[:alpha:]]*?[.][[:alpha:]]*?>))+[|]?$`
	validator, err := regexp.Compile(validExp)
	if err != nil {
		fmt.Printf("Error compiling regex %v: %v", validExp, err)
		return
	}
	piecer, err := regexp.Compile(pieceExp)
	if err != nil {
		fmt.Printf("Error compiling regex %v: %v", pieceExp, err)
		return
	}

	if !validator.MatchString(query) {
		fmt.Println("Query is incorrect: %s", query)
		return
	}

	pieces := piecer.FindAllStringSubmatch(query, -1)
	constraints = make([]string, 0, len(pieces))
	for _, groups := range pieces {
		var piece = groups[1]
		if piece == "" {
			piece = groups[2]
		}
		piece = strings.ToUpper(piece)
		constraints = append(constraints, piece)
	}
	return
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

func main() {
	flag.Parse()
	if *Line > 8 {
		*Line = 16 - *Line
	}
	available := make(map[byte]int)
	isAvailable := func(seq []byte, draws []bool) bool {
		if len(available) == 0 {
			return true // All are available if we didn't specify tiles
		}
		// Special case for "anything".
		if len(draws) == 0 {
			draws = make([]bool, len(seq))
			for i := 0; i < len(draws); i++ {
				draws[i] = true
			}
		}
		// Copy availability
		remaining := make(map[byte]int, len(available))
		for k, v := range available {
			remaining[k] = v
		}
		for i, d := range draws {
			if d {
				c := seq[i]
				if remaining[c] == 0 {
					c = byte('.')
					if remaining[c] == 0 {
						return false
					}
				}
				remaining[c]--
			}
		}
		return true
	}

	query := flag.Arg(0)
	if flag.NArg() > 1 {
		query = flag.Arg(1)
		fmt.Println("Available:", flag.Arg(0))
		for _, ch := range strings.ToUpper(flag.Arg(0)) {
			available[byte(ch)]++
		}
	}
	fmt.Println("Query:", query)

	fmt.Print("Reading recognizer...")
	mFile, err := os.Open("wordswithfriends.mealy")
	if err != nil {
		log.Fatal(err)
	}
	defer mFile.Close()
	recognizer, err := mealy.ReadFrom(mFile)
	if err != nil {
		log.Fatal(err)
	}
	idx := index.Index{recognizer}
	fmt.Println("DONE")

	if len(query) == 0 && len(available) != 0 {
		// Special case for beginning board - give us all available words.
		for seq := range idx.AllSequences() {
			if isAvailable(seq, []bool{}) {
				fmt.Println(SeqScore(seq, 8, 8, []bool{}), string(seq))
			}
		}
		return
	}

	queryPieces := ParseQuery(query)
	if len(queryPieces) == 0 {
		fmt.Println("Query could not be parsed. Quitting.")
		return
	}

	foundAny := false
	allowedInfo := idx.GetAllowedLetters(queryPieces, available)
	for left := range GetSubSuffixes(allowedInfo) {
		subinfo := allowedInfo.MakeSuffix(left)
		if !subinfo.PossiblePrefix() {
			continue
		}
		fmt.Printf("Suff (%d) %v\n", left, subinfo)
		for seq := range idx.ConstrainedSequences(subinfo) {
			if isAvailable(seq, subinfo.Draws[:len(seq)]) {
				foundAny = true
				word := strings.ToUpper(string(FormatSeq(seq, left, len(allowedInfo.Constraints))))
				fmt.Printf("% 3d  %v\n", SeqScore(seq, *Line, left+1, subinfo.Draws[:len(seq)]), word)
			}
		}
	}
	if !foundAny {
		fmt.Println("No possible words found")
	}
}
