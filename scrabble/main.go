package main

import (
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
	mealy.BaseConstraints  // Inherit base "always true" methods
	Query string  // Strong supposition that strings are segmented at byte intervals
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
	fmt.Println("Query:", query)
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
//
// In total, we return
//
// (possible, boundLeft, boundRight, draws, allowedLetters)
func (idx Index) GetAllowedLetters(queryPieces []string) (bool, bool, bool, []bool, []string) {
	// Now for each entry, find a list of letters that can work. Note that
	// we don't test for non-replacement, here. If there is a '.' in the
	// group, we'll get all letters. That may need to be optimized later,
	// but it doesn't seem super likely. The next pass, over actually
	// discovered words, will eliminate things based on replaceability.
	allowed := make([]string, 0, len(queryPieces))
	draws := make([]bool, 0, len(queryPieces))
	possible := true
	boundLeft := false
	boundRight := false
	for i, qp := range queryPieces {
		switch {
		case qp == "|":
			if i == 0 {
				boundLeft = true
			} else {
				boundRight = true
			}
		case qp == ".":
			draws = append(draws, true)
			allowed = append(allowed, qp)
		case len(qp) == 1:
			draws = append(draws, false)
			allowed = append(allowed, qp)
		default:
			found := idx.ValidMissingLetters(qp)
			if len(found) == 0 {
				found = "~"
				possible = false
			}
			draws = append(draws, true)
			allowed = append(allowed, found)
		}
	}
	return possible, boundLeft, boundRight, draws, allowed
}

func MakeRegexp(boundLeft, boundRight bool, draws []bool, allowed []string) (*regexp.Regexp, error) {
	// Now actually search the word list for words that correspond to this,
	// using a regular expression.
	size := len(allowed)
	if boundLeft {
		size++
	}
	if boundRight {
		size++
	}
	clauses := make([]string, size)
	offset := 0
	if boundLeft {
		clauses[0] = "^"
		offset = 1
	}
	if boundRight {
		clauses[len(clauses) - 1] = "$"
	}
	for i, s := range allowed {
		if s == "." || !draws[i] {
			clauses[i + offset] = s
		} else {
			clauses[i + offset] = fmt.Sprintf("[%v]", s)
		}
	}
	clauseStr := strings.Join(clauses, "")
	allowedExp, err := regexp.Compile(clauseStr)
	if err != nil {
		fmt.Printf("Failed to parse expression %v\n", clauseStr)
	}
	return allowedExp, err
}

func GetSubConstraints(draws []bool, allowed []string) (<-chan []string) {
	out := make(chan []string)
	go func() {
		defer close(out)
		for left := 0; left < len(allowed); left++ {
			right := len(allowed)
			out <- allowed[left:right]
			for right := left + 1; right < len(allowed); right++ {
			// TODO: peel off all of the right peelables, and emit the resulting string one at a time.
			// TODO: before emitting, check that there is at least one draw and one non-draw.
			}
			// If "left" is a non-draw index (fixed position), then we advance
			// until it isn't anymore. Otherwise, it's fine to just peel this
			// off (you can always peel off a draw on the left side.
			for !draws[left] {
				left++
			}
		}
	}()
	return out
}


func main() {
	available := make(map[string]int)
	query := os.Args[1]
	if len(os.Args) > 2 {
		query = os.Args[2]
		for _, ch := range strings.Split(os.Args[1], "") {
			ch = strings.ToUpper(ch)
			available[ch]++
		}
	}

	queryPieces := ParseQuery(query)
	if len(queryPieces) == 0 {
		fmt.Println("Query could not be parsed. Quitting.")
		return
	}

	fmt.Print("Reading recognizer...")
	mFile, err := os.Open("TWL06.mealy")
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

	possible, boundLeft, boundRight, draws, allowed := index.GetAllowedLetters(queryPieces)
	if !possible {
		fmt.Printf("Impossible because of nearby letter constraints at '~': %v.\n", allowed)
		return
	}

	allowedExp, err := MakeRegexp(boundLeft, boundRight, draws, allowed)
	if err != nil {
		fmt.Printf("Error getting regular expresison for %v", allowed)
		return
	}
	fmt.Printf("Match Expression: %v\n", allowedExp)

	matchedWords := []MatchedWord{}
	// TODO: Note the "AllSequences" call here? That's not a good plan.
	// Instead, we should construct a series of constraints that will work, and
	// try them all in turn.
	for seq := range index.AllSequences() {
		word := strings.ToUpper(string(seq))
		loc := allowedExp.FindStringIndex(word)
		if loc == nil {
			continue
		}
		// Figure out a comprehensive list of needed letters from the
		// prefix, suffix, and draw indices.
		m := MatchedWord{
			Word:   word,
			Match:  word[loc[0]:loc[1]],
			Prefix: word[:loc[0]],
			Suffix: word[loc[1]:],
		}
		needed := make(map[string]int, len(draws))
		for ci, ch := range m.Match {
			if draws[ci] {
				needed[string(ch)]++
			}
		}
		for _, ch := range m.Prefix {
			needed[string(ch)]++
		}
		for _, ch := range m.Suffix {
			needed[string(ch)]++
		}
		m.Needed = needed

		matchedWords = append(matchedWords, m)
	}
	if len(matchedWords) == 0 {
		fmt.Println("No words matched.")
		return
	}

	// Now we try to assign from our pool of possible letters, to each of
	// the not-fully-constrained bits in our words, skipping over the
	// endpoint notifiers. The draws indicate which entries are used
	// to draw from our available letters.

	if len(available) == 0 {
		for _, w := range matchedWords {
			fmt.Println(w)
		}
		return
	}

	// We have a limited supply - use only what we have, favoring specific
	// letters over blank tiles first.
	workingWords := make([]MatchedWord, 0)
	for _, w := range matchedWords {
		left := make(map[string]int, len(available))
		for k, v := range available {
			left[k] = v
		}
		works := true
		for letter, needed := range w.Needed {
			remaining := left[letter] - needed
			if remaining < 0 {
				left["."] += remaining
				if left["."] < 0 {
					// We ran out of letters and blanks
					works = false
					break
				}
			}
		}
		if works {
			workingWords = append(workingWords, w)
		}
	}
	if len(workingWords) == 0 {
		fmt.Println("No words worked with your available tiles.")
		return
	}
	for _, w := range workingWords {
		fmt.Println(w.Word)
	}
}
