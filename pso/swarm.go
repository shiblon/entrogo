package pso

import (
	"fmt"
	"math/rand"
)

type Swarm struct {
	Particles []*Particle

	Neighborhood Topology
	Updater      UpdateStrategy
	Fitness      FitnessFunction
}

func NewSwarm(neighborhood Topology, updater UpdateStrategy, fitness FitnessFunction, randSeed int64) (swarm *Swarm) {
	swarm = new(Swarm)
	swarm.Particles = make([]*Particle, neighborhood.Size())
	swarm.Neighborhood = neighborhood
	swarm.Updater = updater
	swarm.Fitness = fitness

	seedGenerator := rand.New(rand.NewSource(randSeed))

	// Always call the function in a goroutine, since it may be a lengthy
	// computation. Vector operations aren't especially fast, either, and can
	// be parallelized.
	particles := make(chan *Particle)
	for _ = range swarm.Particles {
		go func() {
			rgen := rand.New(rand.NewSource(seedGenerator.Int63()))
			pos := swarm.Fitness.RandomPos(rgen)
			vel := swarm.Fitness.RandomVel(rgen)
			val := swarm.Fitness.Query(pos)
			particles <- NewParticle(pos, vel, val, rgen)
		}()
	}

	// Wait for them all to complete, and set up our particle list.
	for i := range swarm.Particles {
		swarm.Particles[i] = <-particles
	}
	return swarm
}

func (swarm *Swarm) BatchUpdate() {
	new_updates := make(chan bool)
	for i := range swarm.Particles {
		go func(particle_index int) {
			swarm.Updater.UpdateSingleTempState(swarm, particle_index)
			new_updates <- true
		}(i)
	}

	// Wait for all to complete.
	for _ = range swarm.Particles {
		<-new_updates
	}

	// Now apply any swarm-wide calculations that can't be done without all of
	// the particles being settled on a location first.
	swarm.Updater.UpdateAllStates(swarm)

	// Finally, compute the value for each new position.
	evals_completed := make(chan int)
	for i := range swarm.Particles {
		go func(pidx int) {
			particle := swarm.Particles[pidx]
			particle.TempVal = swarm.Fitness.Query(particle.TempPos)
			evals_completed <- pidx
		}(i)
	}

	// The whole batch has new temporary coordinates and values. Update current
	// state, and conditionaly update best state.
	for _ = range swarm.Particles {
		i := <-evals_completed
		particle := swarm.Particles[i]
		particle.UpdateCur()
		if swarm.Fitness.LessFit(particle.BestVal, particle.Val) {
			particle.UpdateBest()
		}
	}
}

func (swarm Swarm) BestParticle() *Particle {
	best := swarm.Particles[0]
	for i := 1; i < len(swarm.Particles); i++ {
		if swarm.Fitness.LessFit(best.BestVal, swarm.Particles[i].BestVal) {
			best = swarm.Particles[i]
		}
	}
	return best
}

func (swarm Swarm) String() string {
	s := "Particles\n"
	for i := range swarm.Particles {
		s += fmt.Sprintf("  %02d: %v\n", i, swarm.Particles[i])
	}
	return s
}
