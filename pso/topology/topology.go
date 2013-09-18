package topology

import "fmt"

type Topology interface {
	// Size returns the number of particles in this topology.
	Size() int

	// Tick moves the clock forward on this topology. Some of them are dynamic,
	// or can be made more efficient by knowing when the swarm has been
	// updated. For example, the Star topology is more efficient when it can do
	// a single linear pass through all particles after an update and just
	// compute the best 2. Then it has constant-time behavior thereafter.
	Tick()

	// BestNeighbor returns the index of the best neighbor, given a suitable lessFit function.
	BestNeighbor(i int, lessFit func(a, b int) bool) int
}

type Star struct {
	num int

	ready bool // True if we have computed best and second best since the last tick
	best  int  // last best index swarm-wide
	next  int  // last second-best index swarm-wide
	numCalls int // number of times we've tried to find a neighbor since the last tick
}

func NewStar(numParticles int) *Star {
	return &Star{num: numParticles}
}

func (t *Star) Size() int {
	return t.num
}

func (t *Star) Tick() {
	t.ready = false
	t.numCalls = 0
}

func (t *Star) BestNeighbor(i int, lessFit func(a, b int) bool) int {
	t.numCalls++
	if !t.ready {
		t.best = 0
		t.next = 0
		for n := 1; n < t.num; n++ {
			switch {
			case lessFit(t.best, n):
				t.next = t.best
				t.best = n
			case lessFit(t.next, n):
				t.next = n
			}
		}
		t.ready = true
	}
	if t.numCalls > 4*t.num {
		fmt.Printf("Suspicious number of calls since last tick: %d. Did you forget to call Tick?\n", t.numCalls)
	}
	// All computed, now we can just return them based on which particle this is.
	if i == t.best {
		// Just avoid returning self as the best particle.
		return t.next
	}
	return t.best
}

type Ring struct {
	num int
}

func NewRing(numParticles int) *Ring {
	return &Ring{numParticles}
}

func (t *Ring) Tick() {
}

func (t *Ring) Size() int {
	return int(t.num)
}

func (t *Ring) BestNeighbor(i int, lessFit func(a, b int) bool) int {
	best := (i + 1) % t.num
	if t.num >= 3 {
		lower := (i - 1) % t.num
		if lower < 0 {
			lower += t.num // You have crappy negative modulus semantics, Go!
		}
		if lessFit(best, lower) {
			best = lower
		}
	}
	return best
}
