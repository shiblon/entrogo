package pso

import (
	"fmt"
	"math/rand"
	"monson/pso/fitness"
	"monson/pso/vec"
)

type Particle struct {
	// Current state
	Pos, Vel vec.Vec
	Val      float64

	// Current best state
	BestPos vec.Vec
	BestVal float64

	// Additional state
	BestAge int32
	Bounces int32

	// Scratch state
	TempPos, TempVel vec.Vec
	TempVal          float64
	TempBounced      bool

	// Random number generator. Each particle gets its own.
	randGen *rand.Rand
}

func NewParticle(pos, vel vec.Vec, val float64, rgen *rand.Rand) (par *Particle) {
	if len(pos) != len(vel) {
		panic(fmt.Sprintf("Position and velocity vecs have different lengths: %d != %d", len(pos), len(vel)))
	}
	par = &Particle{}
	par.randGen = rgen
	par.Init(pos, vel, val)
	return
}

func (par *Particle) Init(pos, vel vec.Vec, val float64) {
	par.Pos = pos.Copy()
	par.Vel = vel.Copy()
	par.Val = val

	par.BestPos = pos.Copy()
	par.BestVal = val

	par.BestAge = 0
	par.Bounces = 0

	par.TempPos = pos.Copy()
	par.TempVel = vel.Copy()
	par.TempVal = val
}

// Update the current state with the scratch state. This is useful if we are
// doing batch updates and need to compute other particle values based on a
// consistent time slice.
func (par *Particle) UpdateCur() {
	par.Pos.Replace(par.TempPos)
	par.Vel.Replace(par.TempVel)
	par.Val = par.TempVal
	if par.TempBounced {
		par.Bounces++
	}
	par.BestAge++
}

// We have determined that the current position is better than the current
// best. Overwrite the best and reset the best age.
func (par *Particle) UpdateBest() {
	par.BestAge = 0
	par.BestPos.Replace(par.Pos)
	par.BestVal = par.Val
}

// Stringer
func (par Particle) String() string {
	return fmt.Sprintf(
		"  x=%v  x'=%v\n  f=%v\n  bx=%v\n  bf=%v  ba=%v\n  Tx=%v  Tx'=%v\n  Tv=%v  Tb=%v\n  bounces=%v",
		par.Pos, par.Vel, par.Val, par.BestPos, par.BestVal, par.BestAge, par.TempPos, par.TempVel, par.TempVal, par.TempBounced, par.Bounces)
}
type Swarm struct {
	Particles []*Particle

	Neighborhood Topology
	Updater      UpdateStrategy
	Fitness      fitness.Function
}

func NewSwarm(neighborhood Topology, updater UpdateStrategy, fitfunc fitness.Function, randSeed int64) (swarm *Swarm) {
	swarm = new(Swarm)
	swarm.Particles = make([]*Particle, neighborhood.Size())
	swarm.Neighborhood = neighborhood
	swarm.Updater = updater
	swarm.Fitness = fitfunc

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
