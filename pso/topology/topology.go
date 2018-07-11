// Package topology contains definitions of PSO topology that allow for best
// neighbor querying. Topologies can be static or dynamic. Common static
// topologies are provided.
package topology

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
)

// LessFit is a fitness comparator function that operates on particle indices.
// Indicates whether the particle at index a is less fit than the particle at
// index b.
type LessFit func(a, b int) bool

// Topology defines methods for getting size, moving the topology forward, and
// getting the best neighbor's index given a fitness comparator.
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
	BestNeighbor(i int, lessFit LessFit) int
}

// Star graph.
type Star struct {
	num int

	mu sync.Mutex

	ready    bool // True if we have computed best and second best since the last tick
	best     int  // last best index swarm-wide
	next     int  // last second-best index swarm-wide
	numCalls int  // number of times we've tried to find a neighbor since the last tick
}

// NewStar creates a star topology.
func NewStar(numParticles int) *Star {
	return &Star{num: numParticles}
}

// Size returns the number of particles in the swarm.
func (t *Star) Size() int {
	return t.num
}

// Tick does nothing, since Ring is a static topology.
func (t *Star) Tick() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ready = false
	t.numCalls = 0
}

// BestNeighbor returns the most fit particle in the neighborhood of the particle at index i.
func (t *Star) BestNeighbor(i int, lessFit LessFit) int {
	t.mu.Lock()
	defer t.mu.Unlock()
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
		log.Printf("Suspicious number of calls since last tick: %d. Did you forget to call Tick?\n", t.numCalls)
	}
	// All computed, now we can just return them based on which particle this is.
	if i == t.best {
		// Just avoid returning self as the best particle.
		return t.next
	}
	return t.best
}

// Ring graph.
type Ring struct {
	num int
}

// NewRing creates a double-linked ring topology.
func NewRing(numParticles int) *Ring {
	return &Ring{numParticles}
}

// Tick does nothing, since Ring is a static topology.
func (t *Ring) Tick() {
}

// Size returns the number of particles in the swarm.
func (t *Ring) Size() int {
	return t.num
}

// BestNeighbor returns the most fit particle in the neighborhood of the particle at index i.
func (t *Ring) BestNeighbor(i int, lessFit LessFit) int {
	best := (i + 1) % t.num
	if t.num >= 3 {
		lower := (t.num + i - 1) % t.num
		if lessFit(best, lower) {
			best = lower
		}
	}
	return best
}

// RandomExpander changes the connections between particles randomly every time
// it's asked for a best neighbor..
type RandomExpander struct {
	num    int
	degree int
	// rand contains indices in [0, num-1) (to use, add 1 if >= self)
	// This is necessary because 'rand.X' is stateful, and therefore not parallel.
	rand chan int
}

// NewRandomExpander creates a new random expander graph. The degree must be less than the number of particles and greater than zero.
func NewRandomExpander(rsrc rand.Source, numParticles, degree int) (*RandomExpander, error) {
	if degree >= numParticles {
		return nil, fmt.Errorf("Number of RandomExpander out-bound edges (%d) >= particles (%d):", degree, numParticles)
	} else if degree <= 0 {
		return nil, fmt.Errorf("RandomExpander out-bound edges <= 0: %d", degree)
	}

	// Create the random channel and start populating it.
	randchan := make(chan int, degree)
	go func() {
		r := rand.New(rsrc)
		for {
			randchan <- r.Intn(numParticles - 1)
		}
	}()

	return &RandomExpander{
		num:    numParticles,
		degree: degree,
		rand:   randchan,
	}
}

// Tick does nothing, since Ring is a static topology.
func (t *RandomExpander) Tick() {
}

// Size returns the number of particles in the swarm.
func (t *RandomExpander) Size() int {
	return t.num
}

func (t *RandomExpander) randIndex(self int) int {
	v := <-t.rand
	if v >= self {
		v++
	}
	return v
}

// BestNeighbor returns the most fit particle in the neighborhood of the particle at index i.
func (t *RandomExpander) BestNeighbor(p int, lessFit LessFit) int {
	best := t.randIndex(p)
	for i := 1; i < t.degree; i++ {
		n := t.randIndex(p)
		if lessFit(best, n) {
			best = n
		}
	}
	return best
}
