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

type particleUpdateInfo struct {
	i int
	pos, vel VecFloat64
	val float64
}

func NewSwarm(neighborhood Topology, updater UpdateStrategy, fitness FitnessFunction) (swarm *Swarm) {
	swarm = new(Swarm)
	swarm.Particles = make([]*Particle, neighborhood.Size())
	swarm.Neighborhood = neighborhood
	swarm.Updater = updater
	swarm.Fitness = fitness

	// Always call the function in a goroutine, since it may be a lengthy
	// computation. Vector operations aren't especially fast, either, and can
	// be parallelized.
	particles := make(chan *Particle)
	for _ = range swarm.Particles {
		go func() {
			rgen := rand.New(rand.NewSource(rand.Int63()))
			pos := swarm.Fitness.RandomPos(rgen)
			vel := swarm.Fitness.RandomVel(rgen)
			val := swarm.Fitness.Query(pos)
			particles <- NewParticle(pos, vel, val, rgen)
		}()
	}

	// Wait for them all to complete, and set up our particle list.
	for i:= range swarm.Particles {
		swarm.Particles[i] = <-particles
	}
	return swarm
}

func (swarm *Swarm) BatchUpdate() {
	new_updates := make(chan particleUpdateInfo)
	for i := range swarm.Particles {
		go func(particle_index int) {
			particle := swarm.Particles[particle_index]
			informer_indices := swarm.Neighborhood.Informers(particle_index)
			best_i := informer_indices[0]
			for n := 1; n < len(informer_indices); n++ {
				candidate_i := informer_indices[n]
				if swarm.Fitness.LessFit(
					swarm.Particles[best_i].BestVal,
					swarm.Particles[candidate_i].BestVal) {
					best_i = candidate_i
				}
			}
			pos, vel := swarm.Updater.NextState(particle, swarm.Particles[best_i])
			val := swarm.Fitness.Query(pos)
			new_updates <- particleUpdateInfo{
				i: particle_index,
				pos: pos,
				vel: vel,
				val: val,
			}
		}(i)
	}

	// The whole batch has new temporary coordinates and values. Copy them over
	// all at once.
	for _ = range swarm.Particles {
		update_info := <-new_updates
		particle := swarm.Particles[update_info.i]
		particle.UpdateCur(update_info.pos, update_info.vel, update_info.val)
		if swarm.Fitness.LessFit(particle.BestVal, particle.Val) {
			particle.UpdateBest()
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
