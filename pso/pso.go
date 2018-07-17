// Package pso contains the main PSO orchestration algorithms that make swarms
// do their optimization thing.
package pso

import (
	"fmt"
	"math"
	"math/rand"
	"sort"

	"github.com/shiblon/entrogo/fitness"
	"github.com/shiblon/entrogo/pso/particle"
	"github.com/shiblon/entrogo/pso/topology"
	"github.com/shiblon/entrogo/vec"
)

// This generates the momentum multiplier, applied to the previous velocity.
type MomentumFunc func(u Updater, iter int, particle int) float64

// This generates a multiplier for the momentum based on 'tug', which is the
// dot product of the previous velocity vector with the acceleration. This
// tells us if we are trying to move away from the evidence, for example, if
// tug in [-1, 0).
type TugFunc func(dot float64) float64

// Config holds all of the basic configuration for a full particle swarm run.
type Config struct {
	NewRNG           func() rand.Source // creates a new random source
	DecayAdapt       float64            // multiplier applied to soc/cog constants during non-improvement
	DecayRadius      float64            // multiplier applied to radius after each bounce
	Momentum0        float64            // momentum starting point (also used for constant momentum)
	Momentum1        float64            // momentum "endpoint" (e.g., for linear momentum)
	Momentum         MomentumFunc       // produce the current momentum
	Tug              TugFunc            // produce a momentum multiplier based on degree of "tug" toward information.
	SocConst         float64            // initial social constant.
	CogConst         float64            // initial cognitive constant.
	SocLower         float64            // lower bound for social constant.
	CogLower         float64            // lower bound for cognitive constant.
	BackwardAdapt    bool               // allow adaptation of negative lower bounds based on whole-swarm surprises.
	VelCapMultiplier float64            // maximum velocity to allow as a function of the function's domain diagonal.
	RadiusMultiplier float64            // how much to decay the radius when bouncing.
	BounceMultiplier float64            // how much further to bounce out than usual.
}

// NewBasicConfig creates a basic PSO configuration with fairly useful
// parameters (decaying soc/cog adaptation for non-improvement, bouncing with
// radius decay and distance adjustment, and constant momentum).
func NewBasicConfig(newRNG func() rand.Source) *Config {
	c := &Config{
		NewRNG:           newRNG,
		DecayAdapt:       0.999,
		DecayRadius:      0.9,
		Momentum0:        0.75,
		Momentum1:        0.4,
		SocConst:         2.05,
		CogConst:         2.05,
		SocLower:         0.0,
		CogLower:         0.0,
		VelCapMultiplier: 0.5,
		RadiusMultiplier: 0.1,
		BounceMultiplier: 1.0,
	}
	// Default momentum is constant.
	c.Momentum = func(u Updater, iter int, particle int) float64 {
		return c.Momentum0
	}
	// Default tug function just leaves momentum alone (multiplier of 1.0).
	c.Tug = func(dot float64) float64 {
		return 1.0
	}
	return c
}

// Updater is used to manage a swarm from one moment to the next. The central
// part is the Update function, which causes the clock to tick.
type Updater interface {
	// Initialized tells us whether the swarm is yet at t0.
	Initialized() bool

	// Swarm returns all of the swarm particles.
	Swarm() []*particle.Particle

	// Update ticks the clock and returns the number of function evaluations.
	Update() int

	// BestParticle returns the particle with the fittest BestVal.
	BestParticle() *particle.Particle

	// Batches returns number of improved batches and total batches.
	Batches() (improved, total int)
}

// StandardUpdater is a PSO updater that uses (essentially) standard update
// equations (momentum-based), a fixed topology and number of particles, and a
// single-objective static fitness function.
type StandardUpdater struct {
	Topology topology.Topology
	Fitness  fitness.Function
	Conf     *Config

	swarm          []*particle.Particle
	initialized    bool
	domainDiameter float64
	totalEvals     int
	totalBatches   int
	totalImproved  int

	printChan chan string
}

// NewStandardPSO creates an updater that performs the "standard" optimization.
// This means that the topology is basically fixed at initialization, the
// number of particles doesn't change, the basic update equations are used
// (acceleration, velocity, etc.), and a single-objective static function is
// being optimized.
//
// Note that this begins life without a swarm. The swarm springs into existence
// on the first call to Update.
func NewStandardPSO(t topology.Topology, f fitness.Function, c *Config) *StandardUpdater {
	updater := &StandardUpdater{
		Topology:       t,
		Fitness:        f,
		Conf:           c,
		domainDiameter: f.Diameter(),
		printChan:      make(chan string),
	}

	go func() {
		for {
			fmt.Println(<-updater.printChan)
		}
	}()

	return updater
}

// Initialized returns true if the swarm has reached t0 and the initial states
// have been evaluated for fitness.
func (u *StandardUpdater) Initialized() bool {
	return u.initialized
}

// Swarm returns a list of particles.
func (u *StandardUpdater) Swarm() []*particle.Particle {
	return u.swarm
}

// BestParticle returns the particle with the fittest BestVal.
func (u *StandardUpdater) BestParticle() *particle.Particle {
	best := u.swarm[0]
	for _, p := range u.swarm[1:] {
		if u.Fitness.LessFit(best.BestVal, p.BestVal) {
			best = p
		}
	}
	return best
}

// Batches returns the number of improved batches and the total batches.
func (u *StandardUpdater) Batches() (improved, total int) {
	return u.totalImproved, u.totalBatches
}

// init creates all of the particles in the swarm and evaluates the fitness function
// for all of them. Returns the number of function evaluations needed.
func (u *StandardUpdater) init() int {
	u.swarm = make([]*particle.Particle, u.Topology.Size())

	// Evaluate the function concurrently.
	particles := make(chan *particle.Particle)
	for i := range u.swarm {
		go func() {
			par := particle.NewRandomParticle(u.Conf.NewRNG(), i, u.Fitness)
			par.ResetVal(u.Fitness.Query(par.Pos))
			particles <- par
		}()
	}

	// Set up the particles.
	for i := range u.swarm {
		u.swarm[i] = <-particles
	}

	u.initialized = true
	return len(u.swarm)
}

// Update moves the swarm from one time slice to another. The first call moves
// the swarm to t[0] by initializing it. After that it ticks the clock with each call.
// Returns the number of function evaluations performed.
func (u *StandardUpdater) Update() int {
	bestUpdated := false
	defer func() {
		u.Topology.Tick()
		u.totalBatches++
		if bestUpdated {
			u.totalImproved++
		}
	}()

	if !u.Initialized() {
		u.totalImproved++
		bestUpdated = true
		return u.init()
	}

	// First let all particles move based on their favorite neighbor.
	done := make(chan bool, len(u.swarm))
	for pidx := range u.swarm {
		pidx := pidx
		go func() {
			u.moveOneParticle(pidx)
			done <- true
		}()
	}

	// Wait for them to finish.
	for _ = range u.swarm {
		<-done
	}

	// TODO: perhaps just return markers indicating what needs to happen next, then
	// update all of the states after the fact.
	u.bounceAll()

	// Evaluate the function and update current and best states.
	evaluated := make(chan int)
	for i := range u.swarm {
		go func(pidx int) {
			p := u.swarm[pidx]
			p.Scratch().Val = u.Fitness.Query(p.Scratch().Pos)
			p.UpdateCur()
			if u.Fitness.LessFit(p.BestVal, p.Val) {
				bestUpdated = true
				p.UpdateBest()
			}
			evaluated <- 1
		}(i)
	}

	// Wait for it to finish.
	num_evaluations := 0
	for _ = range u.swarm {
		num_evaluations += <-evaluated
	}

	u.totalEvals += num_evaluations
	return num_evaluations
}

func (u *StandardUpdater) momentum(particle *particle.Particle, dot float64) float64 {
	return u.Conf.Momentum(u, u.totalEvals, particle.Id) * u.Conf.Tug(dot)
}

func (u *StandardUpdater) topoLessFit(a, b int) bool {
	return u.Fitness.LessFit(u.swarm[a].BestVal, u.swarm[b].BestVal)
}

func (u *StandardUpdater) moveOneParticle(pidx int) {
	adapt := u.Conf.DecayAdapt
	soc := u.Conf.SocConst
	cog := u.Conf.CogConst
	socLower := u.Conf.SocLower
	cogLower := u.Conf.CogLower
	maxvel_fraction := u.Conf.VelCapMultiplier

	p := u.swarm[pidx]

	informer := u.swarm[u.Topology.BestNeighbor(pidx, u.topoLessFit)]
	dims := len(p.Pos)

	if adapt != 1.0 {
		socFactor := math.Pow(adapt, float64(informer.T-informer.BestT))
		cogFactor := math.Pow(adapt, float64(p.T-p.BestT))
		soc *= socFactor
		socLower *= socFactor
		cog *= cogFactor
		cogLower *= cogFactor
	}

	if u.Conf.BackwardAdapt {
		var particles []*particle.Particle
		for i := 0; i < len(u.swarm); i++ {
			particles = append(particles, u.swarm[i])
		}
		// Sort by descending fitness.
		sort.Slice(particles, func(a, b int) bool {
			return u.Fitness.LessFit(particles[b].BestVal, particles[a].BestVal)
		})
		// Best is now on top. Compute distances and look for surprises (descending distance).
		numDescents := 0
		best := particles[0]
		curr := 0.0
		for _, p := range particles {
			dist := p.BestPos.Sub(best.BestPos).Norm(2)
			if dist < curr {
				numDescents++
			}
			curr = dist
		}
		// Surprises / (P-2) gives the estimated amount of non-convexity:
		nonConvexity := float64(numDescents) / float64(len(u.swarm)-2)

		// TODO: determine whether to *also* slide the top down

		// We can use our non-convexity estimate to see how negative we should go with cognition.
		if socLower < 0 {
			socLower *= nonConvexity
		}
		if cogLower < 0 {
			cogLower *= nonConvexity
		}
	}

	rand_soc := vec.NewFFilled(dims, p.Rand().Float64).SMulBy(soc - socLower).SAddBy(socLower)
	rand_cog := vec.NewFFilled(dims, p.Rand().Float64).SMulBy(cog - cogLower).SAddBy(cogLower)

	to_informer := informer.BestPos.Sub(p.Pos).MulBy(rand_soc)
	acc := p.BestPos.Sub(p.Pos).MulBy(rand_cog).AddBy(to_informer)

	// Cap the velocity if necessary.
	scratch := p.Scratch()

	dot := p.Vel.Normalized().Dot(acc.Normalized())
	// u.printChan <- fmt.Sprintf("tug=%v", dot)

	scratch.Vel.Replace(p.Vel).SMulBy(u.momentum(p, dot)).AddBy(acc)

	sl := u.Fitness.SideLengths()
	for i, v := range scratch.Vel {
		mv := maxvel_fraction * sl[i]
		if v > mv {
			scratch.Vel[i] = mv
		} else if v < -mv {
			scratch.Vel[i] = -mv
		}
	}

	scratch.Pos.Replace(p.Pos).AddBy(scratch.Vel)
	scratch.Bounced = false
}

func (u *StandardUpdater) bounceAll() {
	radius := u.Conf.RadiusMultiplier * u.domainDiameter
	bounce_factor := u.Conf.DecayRadius

	if radius <= 0.0 {
		return
	}

	factors := vec.New(len(u.swarm)).MapBy(
		func(i int, _ float64) float64 {
			return math.Pow(bounce_factor, float64(u.swarm[i].Bounces))
		})
	// TODO: implement in go routines. Note that we'd need to protect the
	// "other" particle if we try to bounce it, because we may have a race
	// condition with it.
	for i, p := range u.swarm {
		if p.Scratch().Bounced {
			// TODO: determine whether we want to do it this way.
			// This approach means that a particle only ever collides with
			// one other, which means that all overlaps are not detected.
			// That is probably the best approach, because that means that
			// some particles can still occupy that space.
			continue
		}
		for n := i + 1; n < len(u.swarm); n++ {
			other := u.swarm[n]
			test_dist := (factors[i] + factors[n]) * radius
			real_dist := other.Scratch().Pos.Sub(p.Scratch().Pos).Mag()
			if real_dist < test_dist {
				if !p.Scratch().Bounced {
					u.doBounce(p, 1.0/factors[i])
				}
				if !other.Scratch().Bounced {
					u.doBounce(other, 1.0/factors[n])
				}
			}
		}
	}
}

func (u *StandardUpdater) doBounce(particle *particle.Particle, springiness float64) {
	bounceBy := springiness * u.Conf.BounceMultiplier
	particle.Scratch().Vel.Negate()
	particle.Scratch().Pos.SMulBy(bounceBy).Negate().AddBy(particle.Pos.SMul(1 + bounceBy))
	particle.Scratch().Bounced = true
}
