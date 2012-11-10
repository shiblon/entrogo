package mealy

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"sort"
)
// Transitions are 32-bit integers split up thus:
//
// - 8 bits: trigger value (a byte) - first to make sorting work as expected.
//
// - 23 bits: next state ID (We can thus handle a little over 8 million states).
//
// - 1 bit: terminal flag
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

// Create a unique and deterministic fingerprint for this state.
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

