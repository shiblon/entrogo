package pso

import (
	"captcha/fitness"
	"captcha/pso/particle"
	"captcha/pso/topology"
	"code.google.com/p/entrogo/vec"
	"fmt"
	"math"
)

// This generates the momentum multiplier, applied to the previous velocity.
type MomentumFunc func(u Updater, iter int, particle int) float64

// This generates a multiplier for the momentum based on 'tug', which is the
// dot product of the previous velocity vector with the acceleration. This
// tells us if we are trying to move away from the evidence, for example, if
// tug in [-1, 0).
type TugFunc func(dot float64) float64

type Config struct {
	DecayAdapt       float64
	DecayRadius      float64
	Momentum0        float64
	Momentum1        float64
	Momentum         MomentumFunc
	Tug              TugFunc
	SocConst         float64
	CogConst         float64
	SocLower         float64
	CogLower         float64
	VelCapMultiplier float64
	RadiusMultiplier float64
	BounceMultiplier float64 // how much further to bounce out than usual.
}

func NewBasicConfig() *Config {
	c := &Config{
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

type standardUpdater struct {
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
// Note that this has no swarm attached. It has what it needs to create one,
// but the swarm itself is nil at this stage.
func NewStandardPSO(t topology.Topology, f fitness.Function, c *Config) Updater {
	updater := &standardUpdater{
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
func (u *standardUpdater) Initialized() bool {
	return u.initialized
}

// Swarm returns a list of particles.
func (u *standardUpdater) Swarm() []*particle.Particle {
	return u.swarm
}

// BestParticle returns the particle with the fittest BestVal.
func (u *standardUpdater) BestParticle() *particle.Particle {
	best := u.swarm[0]
	for _, p := range u.swarm[1:] {
		if u.Fitness.LessFit(best.BestVal, p.BestVal) {
			best = p
		}
	}
	return best
}

// Batches returns the number of improved batches and the total batches.
func (u *standardUpdater) Batches() (improved, total int) {
	return u.totalImproved, u.totalBatches
}

// init creates all of the particles in the swarm and evaluates the fitness function
// for all of them. Returns the number of function evaluations needed.
func (u *standardUpdater) init() int {
	u.swarm = make([]*particle.Particle, u.Topology.Size())

	// Evaluate the function concurrently.
	particles := make(chan *particle.Particle)
	for i := range u.swarm {
		go func() {
			par := particle.NewRandomParticle(i, u.Fitness)
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
func (u *standardUpdater) Update() int {
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

func (u *standardUpdater) momentum(particle *particle.Particle, dot float64) float64 {
	return u.Conf.Momentum(u, u.totalEvals, particle.Id) * u.Conf.Tug(dot)
}

func (u *standardUpdater) topoLessFit(a, b int) bool {
	return u.Fitness.LessFit(u.swarm[a].BestVal, u.swarm[b].BestVal)
}

func (u *standardUpdater) moveOneParticle(pidx int) {
	adapt := u.Conf.DecayAdapt
	soc := u.Conf.SocConst
	cog := u.Conf.CogConst
	socLower := u.Conf.SocLower
	cogLower := u.Conf.CogLower
	maxvel_fraction := u.Conf.VelCapMultiplier

	particle := u.swarm[pidx]

	informer := u.swarm[u.Topology.BestNeighbor(pidx, u.topoLessFit)]
	dims := len(particle.Pos)

	if adapt != 1.0 {
		socFactor := math.Pow(adapt, float64(informer.T-informer.BestT))
		cogFactor := math.Pow(adapt, float64(particle.T-particle.BestT))
		soc *= socFactor
		socLower *= socFactor
		cog *= cogFactor
		cogLower *= cogFactor
	}

	rand_soc := vec.NewFFilled(dims, particle.Rand().Float64).SMulBy(soc - socLower).SAddBy(socLower)
	rand_cog := vec.NewFFilled(dims, particle.Rand().Float64).SMulBy(cog - cogLower).SAddBy(cogLower)

	to_informer := informer.BestPos.Sub(particle.Pos).MulBy(rand_soc)
	acc := particle.BestPos.Sub(particle.Pos).MulBy(rand_cog).AddBy(to_informer)

	// Cap the velocity if necessary.
	scratch := particle.Scratch()

	dot := particle.Vel.Normalized().Dot(acc.Normalized())
	u.printChan <- fmt.Sprintf("tug=%v", dot)

	scratch.Vel.Replace(particle.Vel).SMulBy(u.momentum(particle, dot)).AddBy(acc)

	sl := u.Fitness.SideLengths()
	for i, v := range scratch.Vel {
		mv := maxvel_fraction * sl[i]
		if v > mv {
			scratch.Vel[i] = mv
		} else if v < -mv {
			scratch.Vel[i] = -mv
		}
	}

	scratch.Pos.Replace(particle.Pos).AddBy(scratch.Vel)
	scratch.Bounced = false
}

func (u *standardUpdater) bounceAll() {
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

func (u *standardUpdater) doBounce(particle *particle.Particle, springiness float64) {
	bounceBy := springiness * u.Conf.BounceMultiplier
	particle.Scratch().Vel.Negate()
	particle.Scratch().Pos.SMulBy(bounceBy).Negate().AddBy(particle.Pos.SMul(1 + bounceBy))
	particle.Scratch().Bounced = true
}
