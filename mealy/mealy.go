// vim: noet sw=4 sts=4 ts=4
package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"sort"
	"strings"
)

type RuneSlice []rune

func (p RuneSlice) Len() int {
	return len(p)
}
func (p RuneSlice) Less(i, j int) bool {
	return p[i] < p[j]
}
func (p RuneSlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

type Transition struct {
	NextState int
	Character rune
	Terminal  bool
}

func (t Transition) Fingerprint() string {
	return fmt.Sprintf("%d:%c:%t", t.NextState, t.Character, t.Terminal)
}
func (t Transition) String() string {
	return t.Fingerprint()
}

type State struct {
	ID          int
	transitions map[rune]Transition
}

func (s *State) String() string {
	str := fmt.Sprintf("State %d:\n", s.ID)
	for _, t := range s.transitions {
		str += fmt.Sprintf("\t%v\n", t)
	}
	return str
}

type DuplicateRuneError rune

func (e DuplicateRuneError) Error() string {
	return fmt.Sprintf("Duplicate rune %r", e)
}

func (s *State) IsEmpty() bool {
	return len(s.transitions) == 0
}

func (s *State) AddTransition(t Transition) error {
	if s.transitions == nil {
		s.transitions = make(map[rune]Transition)
	}
	if _, ok := s.transitions[t.Character]; ok {
		return DuplicateRuneError(t.Character)
	}
	s.transitions[t.Character] = t
	return nil
}

func (s *State) Fingerprint() string {
	// Can't safely memoize the key, since we might add transitions over time.
	// Also, never include the ID - this is meant to be a transition fingerprint.
	keys := make([]string, 0, len(s.transitions))
	for _, v := range s.transitions {
		keys = append(keys, v.Fingerprint())
	}
	sort.Strings(keys)
	h := sha1.New()
	for _, s := range keys {
		io.WriteString(h, s)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (s *State) OutboundRunes() (runes []rune) {
	runes = make([]rune, len(s.transitions))
	i := 0
	for _, v := range s.transitions {
		runes[i] = v.Character
		i++
	}
	sort.Sort(RuneSlice(runes))
	return
}

func CommonPrefixLen(a, b []rune) int {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	return i
}

type MealyMachine struct {
	Start       *State
	States      []*State
	LongestPath int
}

func (m *MealyMachine) BuildFromOrderedWords(words <-chan string) {
	states := make(map[string]*State)

	terminals := []bool{false}
	larvae := []*State{&State{}}

	prefixLength := 0
	prevWord := []rune{}
	newWord := []rune{}

	// Create a new channel that ends with the empty string, to finish the algorithm elegantly.
	terminatingWords := make(chan string)
	go func() {
		defer close(terminatingWords)
		for w := range words {
			terminatingWords <- strings.ToUpper(w)
		}
		terminatingWords <- "" // end with the empty string
	}()

	makeState := func(s *State) (newState *State) {
		if found, ok := states[s.Fingerprint()]; ok {
			newState = found
		} else {
			newState = s
			newState.ID = len(states)
			states[newState.Fingerprint()] = newState
		}
		return
	}

	for word := range terminatingWords {
		newWord = []rune(word)
		if len(newWord) > m.LongestPath {
			m.LongestPath = len(newWord)
		}
		prefixLength = CommonPrefixLen(prevWord, newWord)
		// Back up to the longest common prefix point, making states along the way.
		for i := len(prevWord); i > prefixLength; i-- {
			newState := makeState(larvae[i])
			larvae[i-1].AddTransition(Transition{newState.ID, prevWord[i-1], terminals[i]})
		}
		// Go from first uncommon rune to end of new word, resetting everything (creating new states as needed).
		larvae = larvae[:prefixLength+1]
		terminals = terminals[:prefixLength+1]
		for i := prefixLength + 1; i < len(newWord)+1; i++ {
			larvae = append(larvae, &State{})
			terminals = append(terminals, false)
		}
		terminals[len(newWord)] = true
		prevWord = newWord
	}
	m.Start = makeState(larvae[0])
	m.States = make([]*State, len(states))
	for _, s := range states {
		m.States[s.ID] = s
	}
}

func (m *MealyMachine) Recognizes(word string) bool {
	state := m.Start

	var terminal bool
	for _, r := range []rune(strings.ToUpper(word)) {
		if t, ok := state.transitions[r]; ok {
			state, terminal = m.States[t.NextState], t.Terminal
		} else {
			break
		}
	}
	return state != nil && terminal
}

type stateInfo struct {
	S   *State
	Out []rune
	Cur int
}
func (si *stateInfo) Rune() rune {
	return si.Out[si.Cur]
}
func (si *stateInfo) Exhausted() bool {
	return si.Cur >= len(si.Out)
}
func (si *stateInfo) Advance() {
	si.Cur++
}
func (si *stateInfo) Terminal() bool {
	return si.S.transitions[si.Rune()].Terminal
}
func (si *stateInfo) NextStateId() int {
	return si.S.transitions[si.Rune()].NextState
}

// Return a channel that produces all recognized strings for this machine.
// TODO: Add constraints, where we can force any index to be drawn from a set
// of allowable runes.
// This will affect "Advance" and the initialization of info objects, since
// they'll need a constraint set added, as well as an initial "Cur" set to
// something valid.
func (m *MealyMachine) AllRecognized() <-chan string {
	out := make(chan string)

	// Pop off all of the exhausted states (we've explored all outward paths).
	// Note that only an overflow on the *last* element triggers the popping
	// cascade. Each time a pop occurs, the previous item is incremented,
	// potentially triggering more overflows.
	popExhausted := func(info []stateInfo) []stateInfo {
		size := len(info)
		for size > 0 {
			if !info[size-1].Exhausted() {
				break
			}
			size--
			if size > 0 {
				info[size-1].Advance()
			}
		}
		if size != len(info) {
			info = info[:size]
		}
		return info
	}

	getString := func(info []stateInfo) string {
		runes := make([]rune, len(info))
		for i, si := range info {
			runes[i] = si.Rune()
		}
		return string(runes)
	}

	go func() {
		defer close(out)
		info := append(
			make([]stateInfo, 0, m.LongestPath),
			stateInfo{m.Start, m.Start.OutboundRunes(), 0})

		for info = popExhausted(info); len(info) > 0; info = popExhausted(info) {
			end := &info[len(info)-1]
			if end.Terminal() {
				out <- getString(info)
			}
			if nextState := m.States[end.NextStateId()]; !nextState.IsEmpty() {
				info = append(info, stateInfo{nextState, nextState.OutboundRunes(), 0})
			} else {
				end.Advance()
			}
		}
	}()

	return out
}

func main() {
	words := make(chan string)
	go func() {
		defer close(words)
		words <- "A"
		words <- "AA"
		words <- "AAA"
		words <- "AAB"
		words <- "BAA"
		words <- "CAT"
		words <- "CATERPILLAR"
		words <- "CATERWAL"
		words <- "DOG"
	}()

	m := new(MealyMachine)
	m.BuildFromOrderedWords(words)
	fmt.Println(m)
	fmt.Println(m.Recognizes("BAA"))
	fmt.Println(m.Recognizes("BAB"))
	fmt.Println(m.Recognizes("AAB"))

	for word := range m.AllRecognized() {
		fmt.Println(word)
	}
}
