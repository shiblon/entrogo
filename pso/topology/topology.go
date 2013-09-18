package topology

import (
	"monson/pso/particle"
)

type Topology interface {
	// Size returns the number of particles in this topology.
	Size() int
	// BestNeighbor returns the index of the best neighbor for this particle.
	BestNeighbor(i int, particles []*particle.Particle, lessFit func(a, b float64) bool) int
}

type StarTopology struct {
	num int

	lastTick int // last tick we saw for a particle
	best     int // last best index swarm-wide
	next     int // last second-best index swarm-wide
}

func NewStarTopology(numParticles int) *StarTopology {
	return &StarTopology{num: numParticles}
}

func (t *StarTopology) Size() int {
	return t.num
}

func (t *StarTopology) BestNeighbor(i int, particles []*particle.Particle, lessFit func(a, b float64) bool) int {
	// If we are in a new batch, recompute first and second best values.
	if particles[0].T != t.lastTick {
		t.best = 0
		t.next = 0
		t.lastTick = particles[0].T
		for n := 1; n < len(particles); n++ {
			switch {
			case lessFit(particles[t.best].BestVal, particles[n].BestVal):
				t.next = t.best
				t.best = n
			case lessFit(particles[t.next].BestVal, particles[n].BestVal):
				t.next = n
			}
		}
	}
	// All computed, now we can just return them based on which particle this is.
	if i == t.best {
		// Just avoid returning self as the best particle.
		return t.next
	}
	return t.best
}

type RingTopology struct {
	num int
}

func NewRingTopology(numParticles int) *RingTopology {
	return &RingTopology{numParticles}
}

func (t *RingTopology) Size() int {
	return int(t.num)
}

func (t *RingTopology) BestNeighbor(i int, particles []*particle.Particle, lessFit func(a, b float64) bool) int {
	size := len(particles)

	best := (i + 1) % size
	if size >= 3 {
		lower := (i - 1) % size
		if lower < 0 {
			lower += size // crappy negative modulus semantics, Go!
		}
		if lessFit(particles[best].BestVal, particles[lower].BestVal) {
			best = lower
		}
	}
	return best
}
