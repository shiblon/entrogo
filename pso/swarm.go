package pso

import "fmt"

type Swarm struct {
	Particles []Particle

	Neighborhood Topology
	Updater      UpdateStrategy
	Fitness      FitnessFunction
}

func NewSwarm(neighborhood Topology, updater UpdateStrategy, fitness FitnessFunction) (swarm *Swarm) {
	swarm = new(Swarm)
	swarm.Particles = make([]Particle, neighborhood.Size())
	swarm.Neighborhood = neighborhood
	swarm.Updater = updater
	swarm.Fitness = fitness

	swarm.Init()
	return swarm
}

// Set the particles to initial positions, velocities, and values.
func (swarm *Swarm) Init() {
	for i := range swarm.Particles {
		particle := &swarm.Particles[i]
		particle.Init(swarm.Fitness.RandomPos(), swarm.Fitness.RandomVel())
		// Also query the function and override best value, since this is an initial state.
		particle.Val = swarm.Fitness.Query(particle.Pos)
		particle.BestVal = particle.Val
		particle.TmpVal = particle.Val
	}
}

func (swarm *Swarm) BatchUpdate() {
	for i := range swarm.Particles {
		particle := &swarm.Particles[i]
		// Find the neighbor.
		best_n := 0
		informers := swarm.Neighborhood.Informers(i)
		for n := range informers {
			if swarm.Fitness.LessFit(swarm.Particles[best_n].BestVal, swarm.Particles[n].BestVal) {
				best_n = n
			}
		}
		best_particle_index := informers[best_n]
		swarm.Updater.MoveParticle(particle, swarm.Particles[best_particle_index])
		// Now the TmpPos is set. Call the function for a new value.
		particle.TmpVal = swarm.Fitness.Query(particle.TmpPos)

	}

	// The whole batch has new temporary coordinates and values. Copy them over
	// all at once.
	for i := range swarm.Particles {
		// TODO: there has to be a better way than just silently doing the
		// wrong thing when we forget to take an address...
		particle := &swarm.Particles[i]
		particle.UpdateCur()
		fmt.Println("best, cur:", particle.BestVal, particle.Val)
		if swarm.Fitness.LessFit(particle.BestVal, particle.Val) {
			particle.UpdateBest()
			fmt.Println("Updated best")
		}
	}
}

func (swarm Swarm) String() string {
	s := "Particles\n"
	for i := range swarm.Particles {
		s += fmt.Sprintf("  %02d: %v\n", i, swarm.Particles[i])
	}
	return s
}
