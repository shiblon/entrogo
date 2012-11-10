package mealy

import (
	"bytes"
	"fmt"
)

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

// Return a channel to which all recognized sequences will be sent.
// The channel is closed after the last sequence, making this suitable for use
// in "for range" constructs.
//
// This is an alias for ConstrainedSequences(FullyUnconstrained{}).
func (m *MealyMachine) AllSequences() (out <-chan []byte) {
	return m.ConstrainedSequences(FullyUnconstrained{})
}
