package topology

import (
	"fmt"
	"math/rand"
	"sync"
)

type Comp func(a, b int) bool

var private_lock *sync.Mutex = new(sync.Mutex)

func un(l *sync.Mutex) {
	l.Unlock()
}

func lock() *sync.Mutex {
	private_lock.Lock()
	return private_lock
}

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
	BestNeighbor(i int, lessFit Comp) int
}

// Star graph.
type Star struct {
	num int

	ready    bool // True if we have computed best and second best since the last tick
	best     int  // last best index swarm-wide
	next     int  // last second-best index swarm-wide
	numCalls int  // number of times we've tried to find a neighbor since the last tick
}

func NewStar(numParticles int) *Star {
	return &Star{num: numParticles}
}

func (t *Star) Size() int {
	return t.num
}

func (t *Star) Tick() {
	defer un(lock())
	t.ready = false
	t.numCalls = 0
}

func (t *Star) BestNeighbor(i int, lessFit Comp) int {
	defer un(lock()) // mutable because of how it maintains state
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

// Ring graph.
type Ring struct {
	num int
}

func NewRing(numParticles int) *Ring {
	return &Ring{numParticles}
}

func (t *Ring) Tick() {
}

func (t *Ring) Size() int {
	return t.num
}

func (t *Ring) BestNeighbor(i int, lessFit Comp) int {
	best := (i + 1) % t.num
	if t.num >= 3 {
		lower := (t.num + i - 1) % t.num
		if lessFit(best, lower) {
			best = lower
		}
	}
	return best
}

type RandomExpander struct {
	num    int
	degree int
	// rand contains indices in [0, num-1) (to use, add 1 if >= self)
	// This is necessary because 'rand.X' is stateful, and therefore not parallel.
	rand chan int
}

// NewRandomExpander creates a new random expander graph.
func NewRandomExpander(numParticles, degree int) *RandomExpander {
	if degree >= numParticles {
		panic(fmt.Sprintf("Number of RandomExpander out-bound edges (%d) >= particles (%d):", degree, numParticles))
	} else if degree <= 0 {
		panic(fmt.Sprintf("RandomExpander out-bound edges <= 0: %d", degree))
	}

	// Create the random channel and start populating it.
	randchan := make(chan int, degree)
	go func() {
		r := rand.New(rand.NewSource(rand.Int63()))
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

func (t *RandomExpander) Tick() {
}

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

func (t *RandomExpander) BestNeighbor(p int, lessFit Comp) int {
	best := t.randIndex(p)
	for i := 1; i < t.degree; i++ {
		n := t.randIndex(p)
		if lessFit(best, n) {
			best = n
		}
	}
	return best
}
