package pso

import "fmt"

type Topology interface {
	// Get the size of the (sub)swarm that this applies to.
	Size() int
	// Get a list of informers for this particle index.
	Informers(i int) []int
}

type StarTopology struct {
	Num          int
	allParticles []int
}

func (st StarTopology) Size() int {
	return st.Num
}

func (st StarTopology) Informers(pidx int) (out []int) {
	if pidx < 0 || pidx >= st.Num {
		panic(fmt.Sprintf("Particle index %d out of range", pidx))
	}
	if st.allParticles == nil {
		st.allParticles = make([]int, st.Num)
		for i := range st.allParticles {
			st.allParticles[i] = i
		}
	}
	return st.allParticles
}

type RingTopology int

func (size RingTopology) Size() int {
	return int(size)
}

func (size RingTopology) Informers(pidx int) (out []int) {
	if pidx < 0 || pidx >= int(size) {
		panic(fmt.Sprintf("Particle index %d out of range", pidx))
	}
	// Go has crappy and useless negative modulus semantics.
	lower := (pidx - 1) % int(size)
	if lower < 0 {
		lower += int(size)
	}
	return []int{lower, (pidx + 1) % int(size)}
}
