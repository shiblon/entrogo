package pso

type Swarm struct {
	Dim          int
	Particles    []*Particle
	Neighborhood func(int) []int
}

func NewSwarm(dim, size int, neighborhood func(int) []int) (swarm *Swarm) {
	swarm = new(Swarm)
	swarm.Dim = dim
	swarm.Particles = make([]*Particle, size)
	swarm.Neighborhood = neighborhood

	for i := range swarm.Particles {
		swarm.Particles[i].Init(dim)
	}
	return
}

// Reset the particles to initial positions, velocities, and values.
func (swarm *Swarm) Reset() {
}

func (swarm *Swarm) BatchUpdate() {
	//
}
