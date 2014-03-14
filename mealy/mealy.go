/*
Implements a Mealy Machine as described in the paper at
  http://www.n3labs.com/pdf/lexicon-squeeze.pdf
The machine is defined for byte values, and serializes with that assumption.
*/
package mealy

import (
	"bytes"
	"fmt"
	"sort"
)

type Recognizer []state

// Builds a new mealy machine from an ordered list of values. Keeps working
// until the channel is closed, at which point it finalizes and returns.
func FromChannel(values <-chan []byte) Recognizer {
	self := Recognizer{}

	states := make(map[string]int)
	terminals := []bool{false}
	larvae := []state{state{}}

	prefixLen := 0
	prevValue := []byte{}

	// Find or create a state corresponding to what's passed in.
	makeState := func(s state) (id int) {
		fprint := s.Fingerprint()
		var ok bool
		if id, ok = states[fprint]; !ok {
			id = len(self)
			self = append(self, s)
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
		prefixLen = commonPrefixLen(prevValue, value)
		makeSuffixStates(prefixLen)
		// Go from first uncommon byte to end of new value, resetting
		// everything (creating new states as needed).
		larvae = larvae[:prefixLen+1]
		terminals = terminals[:prefixLen+1]
		for i := prefixLen + 1; i < len(value)+1; i++ {
			larvae = append(larvae, state{})
			terminals = append(terminals, false)
		}
		terminals[len(value)] = true
		prevValue = value
	}

	// Finish up by making all remaining states, then create a start state.
	makeSuffixStates(0)
	if startId := makeState(larvae[0]); startId != len(self)-1 {
		panic(fmt.Sprintf(
			"Unexpected start ID, not at the end: %v < %v",
			startId, len(self)-1))
	}

	// Start state is at len - 1; final state is at 0.
	return self
}

func (self Recognizer) String() string {
	return fmt.Sprintf("%v", []state(self))
}

func (self Recognizer) Start() state {
	return self[len(self)-1]
}

func (self Recognizer) TotalTransitions() int {
	num := 0
	for _, state := range self {
		num += len(state)
	}
	return num
}

func (self Recognizer) UniqueTransitions() int {
	unique := make(map[transition]int)
	for _, state := range self {
		for _, transition := range state {
			unique[transition]++
		}
	}
	return len(unique)
}

func (self Recognizer) MaxStateTransitions() int {
	max := 0
	for _, state := range self {
		n := state.Len()
		if max < n {
			max = n
		}
	}
	return max
}

// Return a sorted slice of all byte values that trigger a transition anywhere.
func (self Recognizer) AllTriggers() []byte {
	triggerMap := make(map[int]bool)
	for _, state := range self {
		for _, transition := range state {
			triggerMap[int(transition.Trigger())] = true
		}
	}
	triggers := make([]int, 0, len(triggerMap))
	for k, _ := range triggerMap {
		triggers = append(triggers, k)
	}
	sort.Ints(triggers)
	byteTriggers := make([]byte, len(triggers))
	for i, t := range triggers {
		byteTriggers[i] = byte(t)
	}
	return byteTriggers
}

func (self Recognizer) Recognizes(value []byte) bool {
	if len(self) == 0 {
		return false
	}

	var tran transition

	state := self.Start()
	for _, v := range value {
		if found := state.IndexForTrigger(v); found < len(state) {
			tran = state[found]
			state = self[tran.ToState()]
		} else {
			break
		}
	}
	return tran.IsTerminal()
}

type pathNode struct {
	s state
	c int
}

func (p pathNode) CurrentTransition() transition {
	return p.s[p.c]
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
	return p.c >= len(p.s)
}
func (p *pathNode) Advance() {
	p.c++
}
func (p *pathNode) AdvanceUntilAllowed(allowed func(byte) bool) {
	for ; p.c < len(p.s); p.c++ {
		if allowed(p.Trigger()) {
			break
		}
	}
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
func (self *Recognizer) ConstrainedSequences(con Constraints) <-chan []byte {
	out := make(chan []byte)

	// Advance the last element of the node path, taking constraints into
	// account.
	advanceUntilAllowed := func(i int, n *pathNode) {
		n.AdvanceUntilAllowed(func(b byte) bool {
			return con.IsValueAllowed(i, b)
		})
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
		path := []pathNode{pathNode{self.Start(), 0}}
		advanceLastUntilAllowed(path) // Needed for node initialization

		for path = popExhausted(path); len(path) > 0; path = popExhausted(path) {
			end := &path[len(path)-1]
			curTransition := end.CurrentTransition()
			if curTransition.IsTerminal() && con.IsLargeEnough(len(path)) {
				if b := getBytes(path); con.IsSequenceAllowed(b) {
					out <- b
				}
			}
			nextState := (*self)[curTransition.ToState()]
			if !nextState.IsEmpty() && con.IsSmallEnough(len(path)+1) {
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

// Return a channel to which all recognized sequences will be sent.
// The channel is closed after the last sequence, making this suitable for use
// in "for range" constructs.
//
// This is an alias for ConstrainedSequences(BaseConstraints{}).
func (self *Recognizer) AllSequences() (out <-chan []byte) {
	return self.ConstrainedSequences(BaseConstraints{})
}
