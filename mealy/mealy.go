package mealy

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"sort"
)

// Transitions are 32-bit integers, and we split them up this way:
// 8 bits: trigger value (a byte) - comes first so that these sort on triggers.
// 23 bits: next state ID (We can thus handle a little over 8 million states).
// 1 bit: terminal
type Transition uint32

// Create a new transition, triggered by "trigger", passing to state
// "toStateId", and with terminal status "isTerminal".
func NewTransition(trigger byte, toStateId int, isTerminal bool) Transition {
	t := uint32(trigger) << 24
	t |= (uint32(toStateId) << 1) & 0xfffffe
	if isTerminal {
		t |= 0x01
	}
	return Transition(t)
}

// Return the value that triggers this transition.
func (t Transition) Trigger() byte {
	return byte(t >> 24)
}

// Get the next State ID from this transition (an integer).
func (t Transition) ToState() int {
	return int(t>>1) & 0x7fffff
}

// Return true if this transition is a terminal transition.
func (t Transition) IsTerminal() bool {
	return (t & 1) != 0
}

// A nice human-readable representation.
func (t Transition) String() string {
	return fmt.Sprintf("%x->%x (%t)", t.Trigger(), t.ToState(), t.IsTerminal())
}

// States are just a (possibly empty) list of transitions to other states.
// Implements the sorting interface.
type State []Transition

func (s State) Len() int {
	return len(s)
}
func (s State) Less(i, j int) bool {
	return s[i] < s[j]
}
func (s State) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Return true if this state has no transitions.
func (s State) IsEmpty() bool {
	return len(s) == 0
}

// Gets a list of trigger values that lead to transitions out of this state.
// Returned in sorted order.
func (s State) Triggers() (triggers []byte) {
	triggers = make([]byte, len(s))
	for i, t := range s {
		triggers[i] = t.Trigger()
	}
	return
}

// Get a unique fingerprint for this state.
func (s State) Fingerprint() string {
	hash := sha1.New()
	for _, transition := range s {
		binary.Write(hash, binary.BigEndian, transition)
	}
	return base64.StdEncoding.EncodeToString(hash.Sum(nil))
}

// Get the index of the transition corresponding to the given trigger value. Returns len(s) if not found.
func (s State) IndexForTrigger(value byte) int {
	i := sort.Search(len(s), func(x int) bool { return s[x].Trigger() >= value })
	if i < len(s) && s[i].Trigger() == value {
		return i
	}
	return len(s)
}

// Add a transition to this state. Keeps them properly ordered.
func (s *State) AddTransition(t Transition) {
	// Insert in sorted order.
	i := sort.Search(len(*s), func(x int) bool { return (*s)[x] >= t })
	if i < len(*s) && (*s)[i] == t {
		// Already there - we're done.
		return
	}
	*s = append(*s, t)
	// TODO: can we do this faster? The problem is that we probably start out
	// at capacity, then append, which copies already, so copying again (to
	// shift things over) isn't necessarily all that helpful.
	sort.Sort((*s)[i:])
}

// A Mealy recognizer is a list of states. We keep track of a few other things,
// too, like the longest path found.
type MealyMachine struct {
	StartID     int
	States      []State
	LongestPath int
}

// Builds a new mealy machine from an ordered list of values. Keeps working
// until the channel is closed, at which point it finalizes and returns.
func NewMealyMachine(values <-chan []byte) MealyMachine {
	m := MealyMachine{}

	states := make(map[string]int)
	terminals := []bool{false}
	larvae := []State{State{}}

	prefixLen := 0
	prevValue := []byte{}

	// Find or create a state corresponding to what's passed in.
	makeState := func(s State) (id int) {
		fprint := s.Fingerprint()
		var ok bool
		if id, ok = states[fprint]; !ok {
			id = len(m.States)
			m.States = append(m.States, s)
			states[fprint] = id
		}
		return
	}

	// Find the longest common prefix length.
	commonPrefixLen := func(a, b []byte) (l int) {
		for l = 0; l < len(a) && l < len(b) && a[l] == b[l]; l++ {
		}
		return
	}

	// Make all states up to but not including the prefix point.
	// Modifies larvae by adding transitions as needed.
	makeSuffixStates := func(p int) {
		for i := len(prevValue); i > p; i-- {
			larvae[i-1].AddTransition(
				NewTransition(prevValue[i-1],
					makeState(larvae[i]),
					terminals[i]))
		}
	}

	for value := range values {
		if bytes.Compare(prevValue, value) >= 0 {
			panic(fmt.Sprintf(
				"Cannot build a Mealy machine from out-of-order "+
					"values: %v : %v\n",
				prevValue, value))
		}
		if len(value) > m.LongestPath {
			m.LongestPath = len(value)
		}
		prefixLen = commonPrefixLen(prevValue, value)
		makeSuffixStates(prefixLen)
		// Go from first uncommon byte to end of new value, resetting
		// everything (creating new states as needed).
		larvae = larvae[:prefixLen+1]
		terminals = terminals[:prefixLen+1]
		for i := prefixLen + 1; i < len(value)+1; i++ {
			larvae = append(larvae, State{})
			terminals = append(terminals, false)
		}
		terminals[len(value)] = true
		prevValue = value
	}

	// Finish up by making all remaining states, then create a start state.
	makeSuffixStates(0)
	m.StartID = makeState(larvae[0])

	return m
}

func (m MealyMachine) Start() State {
	return m.States[m.StartID]
}

func (m *MealyMachine) Recognizes(value []byte) bool {
	if len(m.States) == 0 {
		return false
	}

	var transition Transition

	state := m.Start()
	for _, v := range value {
		if found := state.IndexForTrigger(v); found < len(state) {
			transition = state[found]
			state = m.States[transition.ToState()]
		} else {
			break
		}
	}
	return transition.IsTerminal()
}

type pathNode struct {
	state State
	cur   int
}

func (p pathNode) CurrentTransition() Transition {
	return p.state[p.cur]
}
func (p pathNode) ToState() int {
	return p.CurrentTransition().ToState()
}
func (p pathNode) IsTerminal() bool {
	return p.CurrentTransition().IsTerminal()
}
func (p pathNode) Trigger() byte {
	return p.CurrentTransition().Trigger()
}
func (p pathNode) Exhausted() bool {
	return p.cur >= len(p.state)
}
func (p *pathNode) Advance() {
	p.cur++
}

// Implement this to specify constraints for the Mealy machine output.
//
// To specify a minimum and/or maximum length, implement IsLargeEnough and/or
// IsSmallEnough, respectively. They work the way you would expect: only values
// that are both small enough and large enough will be emitted.
//
// They are separate functions because they are used in different places for
// different kinds of branch cutting, and this cannot be done properly if the
// two bounds are not specified separately.
//
// If there are only some values that are allowed at certain positions, then
// IsValueAllowed should return true for all allowed values and false for all
// others. If all values are allowed, this must return true all the time.
type Constraints interface {
	IsSmallEnough(size int) bool
	IsLargeEnough(size int) bool
	IsValueAllowed(pos int, val byte) bool
}

// Return a channel that produces all recognized sequences for this machine.
// The channel is closed after the last valid sequence, making this suitable
// for use in "for range" constructs.
//
// Constraints are specified by following the Constraints interface above. Not
// all possible constraints can be specified that way, but those that are
// important for branch reduction are. More complex constaints should be
// implemented as a filter on the output, but size and allowed-value
// constraints can be very helpful in reducing the amount of work done by the
// machine to generate sequences.
func (m *MealyMachine) ConstrainedSequences(con Constraints) <-chan []byte {
	out := make(chan []byte)

	// Advance the last element of the node path, taking constraints into
	// account.
	advanceUntilAllowed := func(i int, n *pathNode) {
		for ; n.cur < len(n.state); n.cur++ {
			if con.IsValueAllowed(i, n.Trigger()) {
				break
			}
		}
	}

	advanceLastUntilAllowed := func(path []pathNode) {
		advanceUntilAllowed(len(path)-1, &path[len(path)-1])
	}

	// Pop off all of the exhausted states (we've explored all outward paths).
	// Note that only an overflow on the *last* element triggers the popping
	// cascade. Each time a pop occurs, the previous item is incremented,
	// potentially triggering more overflows.
	popExhausted := func(path []pathNode) []pathNode {
		size := len(path)
		for size > 0 {
			if !path[size-1].Exhausted() {
				break
			}
			size--
			if size > 0 {
				path[size-1].Advance()
				advanceUntilAllowed(size-1, &path[size-1])
			}
		}
		if size != len(path) {
			path = path[:size]
		}
		return path
	}

	getBytes := func(path []pathNode) []byte {
		bytes := make([]byte, len(path))
		for i, node := range path {
			bytes[i] = node.CurrentTransition().Trigger()
		}
		return bytes
	}

	go func() {
		defer close(out)
		path := append(make([]pathNode, 0, m.LongestPath), pathNode{m.Start(), 0})
		advanceLastUntilAllowed(path) // Needed for node initialization

		for path = popExhausted(path); len(path) > 0; path = popExhausted(path) {
			end := &path[len(path)-1]
			curTransition := end.CurrentTransition()
			if curTransition.IsTerminal() && con.IsLargeEnough(len(path)) {
				out <- getBytes(path)
			}
			nextState := m.States[curTransition.ToState()]
			if !nextState.IsEmpty() && con.IsSmallEnough(len(path)) {
				node := pathNode{nextState, 0}
				path = append(path, node)
			} else {
				end.Advance()
			}
			advanceLastUntilAllowed(path) // Needed for advance and init above.
		}
	}()

	return out
}

// A fully unconstrained Constraints implementation. Always returns true for
// all methods.
type FullyUnconstrained struct{}

func (c FullyUnconstrained) IsSmallEnough(int) bool {
	return true
}
func (c FullyUnconstrained) IsLargeEnough(int) bool {
	return true
}
func (c FullyUnconstrained) IsValueAllowed(int, byte) bool {
	return true
}

// Return a channel to which all recognized sequences will be sent.
// The channel is closed after the last sequence, making this suitable for use
// in "for range" constructs.
//
// This is an alias for ConstrainedSequences(FullyUnconstrained{}).
func (m *MealyMachine) AllSequences() (out <-chan []byte) {
	return m.ConstrainedSequences(FullyUnconstrained{})
}
