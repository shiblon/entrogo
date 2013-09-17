package pso

import (
	"math"
	"math/rand"
	"monson/pso/vec"
)

type UpdateStrategy interface {
	// Output the next "blind" state, which is the state that the
	// particle thinks it should have based only on its best
	// neighbor. This is where you would implement the basic PSO
	// motion strategy.
	UpdateSingleTempState(swarm *Swarm, pidx int)
	// Called after all of the particles have updated temp state.
	// Now more global information is available about where every
	// particle has moved.
	UpdateAllStates(swarm *Swarm)
}

type StandardUpdateStrategy struct {
	DomainDiameter float64
}

func newUniformVec(size int, r *rand.Rand) vec.Vec {
	return vec.New(size).MapBy(func(_ int, _ float64) float64 { return r.Float64() })
}

func (us StandardUpdateStrategy) UpdateSingleTempState(swarm *Swarm, pidx int) {
	adapt := 0.999
	momentum := 0.8

	par := swarm.Particles[pidx]
	dims := len(par.Pos)

	informer_indices := swarm.Neighborhood.Informers(pidx)
	best_i := informer_indices[0]
	for n := 1; n < len(informer_indices); n++ {
		candidate_i := informer_indices[n]
		if swarm.Fitness.LessFit(
			swarm.Particles[best_i].BestVal,
			swarm.Particles[candidate_i].BestVal) {
			best_i = candidate_i
		}
	}
	informer := swarm.Particles[best_i]

	soc := 2.05 * math.Pow(adapt, float64(informer.BestAge))
	cog := 2.05 * math.Pow(adapt, float64(par.BestAge))

	rand_soc := newUniformVec(dims, par.randGen).SMulBy(cog)
	rand_cog := newUniformVec(dims, par.randGen).SMulBy(soc)

	to_informer := informer.BestPos.Sub(par.Pos).MulBy(rand_soc)
	// Compute the personal vector and add to the neighbor vector.
	acc := par.BestPos.Sub(par.Pos).MulBy(rand_cog).AddBy(to_informer)
	vel := par.Vel.SMul(momentum).AddBy(acc)
	pos := par.Pos.Add(vel)

	// Cap the velocity if necessary.
	max_vel_component := swarm.Fitness.RoughDomainDiameter()
	for i := range vel {
		v := vel[i]
		m := math.Abs(v)
		if m > max_vel_component {
			if v < 0 {
				v = -max_vel_component
			} else {
				v = max_vel_component
			}
			vel[i] = v
		}
	}

	par.TempPos = pos
	par.TempVel = vel
	par.TempBounced = false
}

func (us StandardUpdateStrategy) doBounce(particle *Particle, springiness float64) {
	particle.TempVel.Negate()
	particle.TempPos.SMulBy(springiness).Negate().AddBy(particle.Pos.SMul(1 + springiness))
	particle.TempBounced = true
}

func (us StandardUpdateStrategy) UpdateAllStates(swarm *Swarm) {
	radius := 0.05 * us.DomainDiameter
	bounce_factor := 0.95

	if radius > 0.0 {
		factors := make([]float64, len(swarm.Particles))
		springs := make([]float64, len(swarm.Particles))
		for i := range swarm.Particles {
			factors[i] = math.Pow(bounce_factor, float64(swarm.Particles[i].Bounces))
			springs[i] = math.Pow(bounce_factor, -float64(swarm.Particles[i].Bounces))
		}
		for i := range swarm.Particles {
			par := swarm.Particles[i]
			for n := i + 1; n < len(swarm.Particles); n++ {
				other := swarm.Particles[n]
				test_dist := (factors[i] + factors[n]) * radius
				real_dist := other.TempPos.Sub(par.TempPos).Norm(2)
				if real_dist < test_dist {
					if !par.TempBounced {
						us.doBounce(par, springs[i])
					}
					if !other.TempBounced {
						us.doBounce(other, springs[n])
					}
				}
			}
		}
	}
}
