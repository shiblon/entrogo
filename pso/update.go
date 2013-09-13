package pso

import "math"

type UpdateStrategy interface {
	// Move the particle "in place" by writing to it scratch.
	MoveParticle(par *Particle, informer Particle)
}

type StandardUpdateStrategy struct {
}

func (us StandardUpdateStrategy) MoveParticle(par *Particle, informer Particle) {
	adapt := 0.999
	soc := 2.01 * math.Pow(adapt, float64(informer.BestAge))
	cog := 2.01 * math.Pow(adapt, float64(par.BestAge))
	momentum := 1.0
	dims := len(par.Pos)

	rand_soc := NewUniformVecFloat64(dims)
	rand_cog := NewUniformVecFloat64(dims)

	to_personal := par.BestPos.Sub(par.Pos)
	to_informer := informer.BestPos.Sub(par.Pos)

	(&to_personal).MulBy(rand_cog).SMulBy(cog)
	(&to_informer).MulBy(rand_soc).SMulBy(soc)

	acc := to_personal.Add(to_informer)

	// Set velocity and position directly in scratch area.
	par.TmpVel.Replace(acc).SMulBy(momentum).IncrBy(acc)
	par.TmpPos.Replace(par.Pos).IncrBy(par.TmpVel)
}
