package pso

import "math"

type UpdateStrategy interface {
	// Move the particle "in place" by writing to it scratch.
	MoveParticle(par *Particle, informer *Particle)
}

type StandardUpdateStrategy struct {
}

func (us StandardUpdateStrategy) MoveParticle(par *Particle, informer *Particle) {
	adapt := 0.999
	soc := 2.01 * math.Pow(adapt, informer.BestAge)
	cog := 2.01 * math.Pow(adapt, par.BestAge)
	momentum := 1.0
	dims := len(par.Pos)

	rand_vec := NewUniformVecFloat64(dims)
	to_personal := par.BestPos.Sub(par.Pos).MulBy(rand_vec).SMulBy(cog)
	rand_vec.FillUniform() // set to a new random vector
	to_informer := informer.BestPos.Sub(par.Pos).MulBy(rand_vec).SMulBy(cog)

	acc := to_personal.Incr(to_informer) // TODO: save memory and allocations by reusing to_personal.
	// Set velocity and position directly in scratch area.
	par.TmpVel.Replace(acc).SMulBy(momentum).IncrBy(acc)
	par.TmpPos.Replace(par.Pos).IncrBy(par.TmpVel)
}
