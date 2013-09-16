package pso

import (
	"math"
)

type UpdateStrategy interface {
	// Move the particle "in place" by writing to it scratch.
	NextState(par, informer *Particle) (pos, vel VecFloat64)
}

type StandardUpdateStrategy struct {
}

func (us StandardUpdateStrategy) NextState(par, informer *Particle) (pos, vel VecFloat64) {
	adapt := 0.999
	soc := 2.05 * math.Pow(adapt, float64(informer.BestAge))
	cog := 2.05 * math.Pow(adapt, float64(par.BestAge))
	momentum := 0.7
	dims := len(par.Pos)

	rand_soc := NewUniformVecFloat64(dims, par.randGen).SMul(cog)
	rand_cog := NewUniformVecFloat64(dims, par.randGen).SMul(soc)

	to_personal := par.BestPos.Sub(par.Pos)
	to_informer := informer.BestPos.Sub(par.Pos)

	(&to_personal).MulBy(rand_cog)
	(&to_informer).MulBy(rand_soc)

	acc := to_personal.Add(to_informer)
	vel = par.Vel.SMul(momentum).Add(acc)
	pos = par.Pos.Add(vel)

	return pos, vel
}
