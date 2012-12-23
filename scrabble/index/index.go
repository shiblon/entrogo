package index

import (
	"fmt"
	"io"
	"log"
	"monson/mealy"
	"sort"
	"strings"
)

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
	mealy.Recognizer
}

func ReadFrom(r io.Reader) (self Index, err error) {
	recognizer, err := mealy.ReadFrom(r)
	return Index{recognizer}, err
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

type UsedStack struct {
	_stack *[]byte
}

func NewUsedStack() UsedStack {
	return UsedStack{&[]byte{}}
}
func (u UsedStack) Push(val byte) {
	*u._stack = append(*u._stack, val)
}
func (u UsedStack) Pop() (val byte) {
	(*u._stack), val = (*u._stack)[:len(*u._stack)-1], (*u._stack)[len(*u._stack)-1]
	return
}
func (u UsedStack) Len() int {
	return len(*u._stack)
}
func (u UsedStack) String() string {
	return fmt.Sprintf("%s", *u._stack)
}

type AllowedInfo struct {
	mealy.BaseConstraints // inherit base constraint methods
	Constraints           []string
	Draws                 []bool
	Available             map[byte]int

	notAnchored          bool
	_used                 UsedStack
}

func NewAllowedInfo(constraints []string, draws []bool, available map[byte]int) AllowedInfo {
	info := AllowedInfo{
		Constraints: make([]string, len(constraints)),
		Draws:       make([]bool, len(draws)),
		Available:   make(map[byte]int, len(available)),
		_used:       NewUsedStack(),
	}
	copy(info.Constraints, constraints)
	copy(info.Draws, draws)
	for k, v := range available {
		info.Available[k] = v
	}
	return info
}

func NewUnanchoredAllowedInfo(constraints []string, draws []bool, available map[byte]int) AllowedInfo {
	info := NewAllowedInfo(constraints, draws, available)
	info.notAnchored = true
	return info
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
	f := NewAllowedInfo
	if info.notAnchored {
		f = NewUnanchoredAllowedInfo
	}
	return f(info.Constraints[left:], info.Draws[left:], info.Available)
}
func (info AllowedInfo) Possible() bool {
	for _, x := range info.Constraints {
		if strings.ContainsRune(x, '~') {
			return false
		}
	}
	return true
}

// Return false if the first non-wild is an impossible constraint.
func (info AllowedInfo) PossiblePrefix() bool {
	for i, d := range info.Draws {
		if !d {
			// A fixed value is OK
			return true
		}
		if strings.ContainsRune(info.Constraints[i], '~') {
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
	if !info.PossiblePrefix() {
		return false
	}
	useAvailable := len(info.Available)+info._used.Len() > 0
	// We have to manage the _used stack and Available map. When 'i' increases,
	// we add to it, and when it decreases, we take away.
	if useAvailable {
		for info._used.Len() > i {
			info.Available[info._used.Pop()]++
		}
		for info._used.Len() < i {
			info._used.Push(byte('_'))
		}
	}
	if !info.Draws[i] {
		return info.Constraints[i] == string(val)
	}
	// It's a drawn tile, so we not only make sure that it's in our expectation
	// set, we also make sure we actually have it.
	if info.Constraints[i] == "~" {
		return false
	}
	if info.Constraints[i] == "." || strings.ContainsRune(info.Constraints[i], rune(val)) {
		if useAvailable {
			use := byte(val)
			if info.Available[use] == 0 {
				use = '.'
				if info.Available[use] == 0 {
					return false
				}
			}
			info.Available[use]--
			info._used.Push(use)
		}
		return true
	}
	return false
}
func (info AllowedInfo) IsSequenceAllowed(seq []byte) bool {
	// Size is okay if it is the whole thing, or if the value just beyond the
	// end is a draw. If the value just beyond the end is fixed, then this is
	// not a valid sequence because it *must* have more on it to fit the board.
	sizeGood := len(seq) == len(info.Constraints) || info.Draws[len(seq)]
	return (info.notAnchored || info.AnchoredSubSequence(0, len(seq))) && sizeGood
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
func (idx Index) GetAllowedLetters(queryPieces []string, available map[byte]int) (info AllowedInfo) {
	// Now for each entry, find a list of letters that can work. Note that
	// we don't test for non-replacement, here. If there is a '.' in the
	// group, we'll get all letters. That may need to be optimized later,
	// but it doesn't seem super likely. The next pass, over actually
	// discovered words, will eliminate things based on replaceability.
	info = NewAllowedInfo(
		make([]string, len(queryPieces)),
		make([]bool, len(queryPieces)),
		available,
	)

	for i, qp := range queryPieces {
		info.Draws[i] = true
		if len(qp) > 1 {
			// Partial constraint (missing letter).
			letters := idx.ValidMissingLetters(qp)
			if len(letters) == 0 {
				letters = "~"
			}
			// fmt.Println("Query:", qp, "\t["+letters+"]")
			qp = letters
		} else if qp != "." {
			info.Draws[i] = false
		}
		info.Constraints[i] = qp
	}
	return
}
