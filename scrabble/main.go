package main

import (
	"flag"
	"fmt"
	"log"
	"monson/mealy"
	"os"
	"regexp"
	"sort"
	"strings"
)

type MatchedWord struct {
	Word   string
	Match  string
	Prefix string
	Suffix string
	Needed map[string]int
}

var (
	Line = flag.Int("line", 0, "Row or column for this request - helps with scoring - 0 to skip.")

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
		"$...#.....#...$",
		".+...$...$...+.",
		"..#...#.#...#..",
		"*..$...+...$..*",
		".#..#.....#..#.",
		"..#..+...+..#..",
		"...*..$.$..*...",
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
			if draws[i] {  // multipliers only apply to placed tiles
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

type MissingLetterConstraint struct {
	mealy.BaseConstraints        // Inherit base "always true" methods
	Query                 string // Strong supposition that strings are segmented at byte intervals
}

func NewMissingLetterConstraint(query string) MissingLetterConstraint {
	return MissingLetterConstraint{Query: query}
}
func (mlc MissingLetterConstraint) IsSmallEnough(size int) bool {
	return size <= len(mlc.Query)
}
func (mlc MissingLetterConstraint) IsLargeEnough(size int) bool {
	return size >= len(mlc.Query)
}
func (mlc MissingLetterConstraint) IsValueAllowed(i int, val byte) bool {
	return mlc.Query[i] == '.' || mlc.Query[i] == val
}

type Index struct {
	mealy.MealyMachine
}

// Return all words that are valid for the given "missing letter" query.
// Queries are just strings with '.' in them. The '.' is not required, in which
// case we'll simply check that a word is actually in the dictionary.
func (idx Index) ValidWords(query string) (allWords []string) {
	con := NewMissingLetterConstraint(strings.ToUpper(query))
	for seq := range idx.ConstrainedSequences(con) {
		allWords = append(allWords, string(seq))
	}
	return
}

// Return all valid *letters* that can take the place of the sole "." in the query.
func (idx Index) ValidMissingLetters(query string) (allLetters string) {
	if strings.Count(query, ".") != 1 {
		log.Fatalf("Invalid missing-letter query - should have exactly one '.': %s", query)
	}
	pos := strings.IndexRune(query, '.')
	letters := map[string]bool{}
	for _, w := range idx.ValidWords(query) {
		letters[string(w[pos])] = true
	}
	ordered := make([]string, 0, len(letters))
	for k, _ := range letters {
		ordered = append(ordered, k)
	}
	sort.Strings(ordered)
	return strings.Join(ordered, "")
}

type AllowedInfo struct {
	mealy.BaseConstraints // inherit base constraint methods
	Constraints           []string
	Draws                 []bool
}

func (info AllowedInfo) AnchoredSubSequence(left, right int) bool {
	num_draw := 0
	num_fixed := 0
	for x := left; x < right; x++ {
		if info.Draws[x] {
			num_draw++
			if info.Constraints[x] != "." {
				num_fixed++
			}
		} else {
			num_fixed++
		}
	}
	return num_draw > 0 && num_fixed > 0
}
func (info AllowedInfo) MakeSuffix(left int) AllowedInfo {
	return AllowedInfo{
		Constraints: info.Constraints[left:],
		Draws:       info.Draws[left:],
	}
}
func (info AllowedInfo) Possible() bool {
	for _, x := range info.Constraints {
		if strings.ContainsRune(x, '~') {
			return false
		}
	}
	return true
}

func (info AllowedInfo) String() string {
	// Now actually search the word list for words that correspond to this,
	// using a regular expression.
	clauses := make([]string, len(info.Constraints))
	for i, s := range info.Constraints {
		if s == "." || !info.Draws[i] {
			clauses[i] = s
		} else {
			clauses[i] = fmt.Sprintf("[%v]", s)
		}
	}
	return strings.Join(clauses, "")
}

// Allow all valid prefixes of this sequence definition.
func (info AllowedInfo) IsSmallEnough(size int) bool {
	return size <= len(info.Constraints)
}
func (info AllowedInfo) IsValueAllowed(i int, val byte) bool {
	return info.Constraints[i] == "." || strings.ContainsRune(info.Constraints[i], rune(val))
}
func (info AllowedInfo) IsSequenceAllowed(seq []byte) bool {
	// Size is okay if it is the whole thing, or if the value just beyond the
	// end is a draw. If the value just beyond the end is fixed, then this is
	// not a valid sequence because it *must* have more on it to fit the board.
	sizeGood := len(seq) == len(info.Constraints) || info.Draws[len(seq)]
	return info.AnchoredSubSequence(0, len(seq)) && sizeGood
}

// Given a query string, produce the sequence of allowed letters (in a string).
// This does not consider the stuff currently allowed in play, just what the
// dictionary allows.
//
// In particular, for each of these, we produce something:
// - A letter: itself
// - A '.': itself (just means "everything")
// - A 'xx.xx' constraint: all letters that make it a word.
//
// Note that this means that some wild information can be lost, particularly in
// instances where a constrained wild has only one letter that can satisfy the
// query. So, we also return a list of booleans indicating whether the tile at
// that location is fixed (false) or drawable (true).
func (idx Index) GetAllowedLetters(queryPieces []string) (info AllowedInfo) {
	// Now for each entry, find a list of letters that can work. Note that
	// we don't test for non-replacement, here. If there is a '.' in the
	// group, we'll get all letters. That may need to be optimized later,
	// but it doesn't seem super likely. The next pass, over actually
	// discovered words, will eliminate things based on replaceability.
	info = AllowedInfo{
		Constraints: make([]string, len(queryPieces)),
		Draws:       make([]bool, len(queryPieces)),
	}

	for i, qp := range queryPieces {
		info.Draws[i] = true
		if len(qp) > 1 {
			// Partial constraint (missing letter).
			letters := idx.ValidMissingLetters(qp)
			if len(letters) == 0 {
				letters = "~"
			}
			fmt.Println("Query:", qp, "\t[" + letters + "]")
			qp = letters
		} else if qp != "." {
			info.Draws[i] = false
		}
		info.Constraints[i] = qp
	}
	return
}

type Endpoints struct {
	left  int
	right int
}

func GetSubSuffixes(info AllowedInfo) <-chan int {
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

func GetSubConstraints(info AllowedInfo) <-chan Endpoints {
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

func main() {
	flag.Parse()
	available := make(map[byte]int)
	isAvailable := func(seq []byte, draws []bool) bool {
		if len(available) == 0 {
			return true // All are available if we didn't specify tiles
		}
		// Special case for "anything".
		if len(draws) == 0 {
			draws = make([]bool, len(seq))
			for i := 0; i < len(draws); i++ {
				draws[i] = true;
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
	mealy, err := mealy.ReadFrom(mFile)
	if err != nil {
		log.Fatal(err)
	}
	index := Index{mealy}
	fmt.Println("DONE")

	if len(query) == 0 && len(available) != 0 {
		// Special case for beginning board - give us all available words.
		for seq := range index.AllSequences() {
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

	formatSeq := func(word []byte, left, origSize int) []byte {
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

	foundAny := false
	allowedInfo := index.GetAllowedLetters(queryPieces)
	for left := range GetSubSuffixes(allowedInfo) {
		subinfo := allowedInfo.MakeSuffix(left)
		fmt.Printf("Suff (%d) %v\n", left, subinfo)
		if !subinfo.Possible() {
			continue
		}
		for seq := range index.ConstrainedSequences(subinfo) {
			if isAvailable(seq, subinfo.Draws[:len(seq)]) {
				foundAny = true
				word := strings.ToUpper(string(formatSeq(seq, left, len(allowedInfo.Constraints))))
				fmt.Printf("% 3d  %v\n", SeqScore(seq, *Line, left+1, subinfo.Draws[:len(seq)]), word)
			}
		}
	}
	if !foundAny {
		fmt.Println("No possible words found")
	}
}
