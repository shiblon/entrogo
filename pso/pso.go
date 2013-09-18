package pso

import (
	"math"
	"monson/pso/fitness"
	"monson/pso/particle"
	"monson/pso/topology"
	"monson/vec"
)

type Config struct {
}

type Updater interface {
	// Initialized tells us whether the swarm is yet at t0.
	Initialized() bool

	// Update ticks the clock and returns the number of function evaluations.
	Update() int

	// BestParticle returns the particle with the fittest BestVal.
	BestParticle() *particle.Particle
}

type standardUpdater struct {
	Swarm    []*particle.Particle
	Topology topology.Topology
	Fitness  fitness.Function

	initialized    bool
	domainDiameter float64
}

// NewStandardPSO creates an updater that performs the "standard" optimization.
// This means that the topology is basically fixed at initialization, the
// number of particles doesn't change, the basic update equations are used
// (acceleration, velocity, etc.), and a single-objective static function is
// being optimized.
//
// Note that this has no swarm attached. It has what it needs to create one,
// but the swarm itself is nil at this stage.
func NewStandardPSO(t topology.Topology, f fitness.Function) Updater {
	return &standardUpdater{
		Topology:       t,
		Fitness:        f,
		domainDiameter: f.RoughDomainDiameter(),
	}
}

// Initialized returns true if the swarm has reached t0 and the initial states
// have been evaluated for fitness.
func (u *standardUpdater) Initialized() bool {
	return u.initialized
}

// BestParticle returns the particle with the fittest BestVal.
func (u *standardUpdater) BestParticle() *particle.Particle {
	best := u.Swarm[0]
	for _, p := range u.Swarm[1:] {
		if u.Fitness.LessFit(best.BestVal, p.BestVal) {
			best = p
		}
	}
	return best
}

// init creates all of the particles in the swarm and evaluates the fitness function
// for all of them.
func (u *standardUpdater) init() {
	u.Swarm = make([]*particle.Particle, u.Topology.Size())

	// Evaluate the function concurrently.
	particles := make(chan *particle.Particle)
	for _ = range u.Swarm {
		go func() { particles <- particle.NewRandomParticle(u.Fitness, true) }()
	}

	// Set up the particles.
	for i := range u.Swarm {
		u.Swarm[i] = <-particles
	}

	u.initialized = true
}

// Update moves the swarm from one time slice to another. The first call moves
// the swarm to t[0] by initializing it. After that it ticks the clock with each call.
func (u *standardUpdater) Update() int {
	if !u.Initialized() {
		u.init()
		return u.Topology.Size()
	}

	// Otherwise, tick the clock again by moving all of the particles and evaluating.

	// First let all particles move based on their favorite neighbor.
	done := make(chan bool)
	for pidx := range u.Swarm {
		go func(i int) {
			u.moveOneParticle(i)
			done <- true
		}(pidx)
	}

	// Wait for them to finish.
	for _ = range u.Swarm {
		<-done
	}

	// TODO: perhaps just return markers indicating what needs to happen next, then
	// update all of the states after the fact.
	u.bounceAll()

	// Evaluate the function and update current and best states.
	evaluated := make(chan int)
	for i := range u.Swarm {
		go func(pidx int) {
			p := u.Swarm[pidx]
			p.TempVal = u.Fitness.Query(p.TempPos)
			p.UpdateCur()
			if u.Fitness.LessFit(p.BestVal, p.Val) {
				p.UpdateBest()
			}
			evaluated <- 1
		}(i)
	}

	// Wait for it to finish.
	num_evaluations := 0
	for _ = range u.Swarm {
		num_evaluations += <-evaluated
	}

	return num_evaluations
}

func (u *standardUpdater) moveOneParticle(pidx int) {
	adapt := 0.999
	momentum := 0.8
	soc := 2.05
	cog := 2.05

	particle := u.Swarm[pidx]
	informer := u.Swarm[u.Topology.BestNeighbor(pidx, u.Swarm, u.Fitness.LessFit)]
	dims := len(particle.Pos)

	if adapt != 1.0 {
		soc *= math.Pow(adapt, float64(informer.T-informer.BestT))
		cog *= math.Pow(adapt, float64(particle.T-particle.BestT))
	}

	rand_soc := vec.NewFFilled(dims, particle.Rand.Float64).SMulBy(soc)
	rand_cog := vec.NewFFilled(dims, particle.Rand.Float64).SMulBy(cog)

	to_informer := informer.BestPos.Sub(particle.Pos).MulBy(rand_soc)
	acc := particle.BestPos.Sub(particle.Pos).MulBy(rand_cog).AddBy(to_informer)

	// Cap the velocity if necessary.
	// TODO: use some multiple of the diameter?
	particle.TempVel.Replace(particle.Vel).SMulBy(momentum).AddBy(acc)
	velmag := particle.TempVel.Mag()
	maxvelmag := 1.5 * u.domainDiameter
	if velmag > 0 && velmag > maxvelmag {
		particle.TempVel.SMulBy(maxvelmag / velmag)
	}
	/*
		// TODO: decide whether to do this instead.
		for i, v := range particle.TempVel {
			m := math.Abs(v)
			if m > u.domainDiameter
				if v < 0 {
					v = -u.domainDiameter
				} else {
					v = u.domainDiameter
				}
				particle.TempVel[i] = v
			}
		}
	*/

	particle.TempPos.Replace(particle.Pos).AddBy(particle.TempVel)
	particle.TempBounced = false
}

func (u *standardUpdater) bounceAll() {
	radius := 0.05 * u.domainDiameter
	bounce_factor := 0.99

	if radius <= 0.0 {
		return
	}

	factors := vec.New(len(u.Swarm)).MapBy(
		func(i int, _ float64) float64 {
			return math.Pow(bounce_factor, float64(u.Swarm[i].Bounces))
		})
	// TODO: implement in go routines. Note that we'd need to protect the
	// "other" particle if we try to bounce it, because we may have a race
	// condition with it.
	for i, p := range u.Swarm {
		if p.TempBounced {
			// TODO: determine whether we want to do it this way.
			// This approach means that a particle only ever collides with
			// one other, which means that all overlaps are not detected.
			// That is probably the best approach, because that means that
			// some particles can still occupy that space.
			continue
		}
		for n := i + 1; n < len(u.Swarm); n++ {
			other := u.Swarm[n]
			test_dist := (factors[i] + factors[n]) * radius
			real_dist := other.TempPos.Sub(p.TempPos).Mag()
			if real_dist < test_dist {
				if !p.TempBounced {
					u.doBounce(p, 1.0/factors[i])
				}
				if !other.TempBounced {
					u.doBounce(other, 1.0/factors[n])
				}
			}
		}
	}
}

func (u *standardUpdater) doBounce(particle *particle.Particle, springiness float64) {
	particle.TempVel.Negate()
	particle.TempPos.SMulBy(springiness).Negate().AddBy(particle.Pos.SMul(1 + springiness))
	particle.TempBounced = true
}
