package pso

import "fmt"

type Topology interface {
	// Get the size of the (sub)swarm that this applies to.
	Size() int
	// Get a list of informers for this particle index.
	Informers(i int) []int
}

type StarTopology struct {
	num         int
	best        int
	second_best int

	topoCache map[int][]int
}

func NewStarTopology(numParticles int) StarTopology {
	return StarTopology{num: numParticles, topoCache: make(map[int][]int)}
}

func (st StarTopology) Size() int {
	return st.num
}

func (st StarTopology) Informers(pidx int) []int {
	if pidx < 0 || pidx >= st.num {
		panic(fmt.Sprintf("Particle index %d out of range", pidx))
	}
	inf, ok := st.topoCache[pidx]
	if !ok {
		inf = make([]int, 0, st.num)
		for i := 0; i < st.num; i++ {
			if i != pidx { // TODO: allow self links?
				inf = append(inf, i)
			}
		}
		st.topoCache[pidx] = inf
	}
	return inf
}

type RingTopology struct {
	num       int
	topoCache map[int][]int
}

func NewRingTopology(numParticles int) RingTopology {
	return RingTopology{numParticles, make(map[int][]int)}
}

func (rt RingTopology) Size() int {
	return int(rt.num)
}

func (rt RingTopology) Informers(pidx int) (out []int) {
	if pidx < 0 || pidx >= rt.num {
		panic(fmt.Sprintf("Particle index %d out of range", pidx))
	}
	inf, ok := rt.topoCache[pidx]
	if !ok {
		inf = make([]int, 2)
		lower := (pidx - 1) % rt.num
		// Go has crappy and useless negative modulus semantics.
		if lower < 0 {
			lower += rt.num
		}
		inf[0] = lower
		inf[1] = (pidx + 1) % rt.num
		rt.topoCache[pidx] = inf
	}
	return inf
}
