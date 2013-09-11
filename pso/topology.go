package pso

type Topology interface {
	// Get a list of informers for this particle index.
	Informers(i int) []int
}

type StarTopology struct {
	NumParticles int
	allParticles []int
}

func (st StarTopology) Informers(pidx int) (out []int) {
	if pidx < 0 || pidx >= st.NumParticles {
		panic(fmt.SPrintf("Particle index %d out of range", pidx))
	}
	if st.allParticles == nil {
		st.allParticles = make([]int, st.NumParticles)
		for i := range st.allParticles {
			st.allParticles[i] = i
		}
	}
	return st.allParticles
}

type RingTopology struct {
	NumParticles int
}

func (rt RingTopology) Informers(pidx int) (out []int) {
	if pidx < 0 || pidx >= rt.NumParticles {
		panic(fmt.SPrintf("Particle index %d out of range", pidx))
	}
	return []int{(pidx - 1) % rt.NumParticles, (pidx + 1) % rt.NumParticles}
}
