package pso

type Swarm struct {
	Dim       int
	Particles []*Particle

	Neighborhood Topology
	Updater      UpdateStrategy
	Fitness      FitnessFunction
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
	// TODO
}

func (swarm *Swarm) BatchUpdate() {
	for i := range swarm.Particles {
		particle := swarm.Particles[i]
		// Find the neighbor.
		best_n := 0
		informers := swarm.Neighborhood.Informers(i)
		for n := range informers {
			if swarm.Fitness.LessFit(swarm.Particles[best_n], swarm.Particles[n]) {
				best_n = n
			}
		}
		swarm.Updater.MoveParticle(particle, swarm.Particle[best_n})

		// Now the TmpPos is set. Call the function for a new value.
		particle.TmpVal = swarm.Fitness.Query(particle.TmpPos)
	}

	// The whole batch has new temporary coordinates and values. Copy them over
	// all at once.
	for i := range swarm.Particles {
		particle = swarm.Particles[i]
		particle.UpdateCur()
		if swarm.Fitness.LessFit(particle.BestVal, particle.Val) {
			particle.UpdateBest()
		}
	}
}
