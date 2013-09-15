package pso

import (
	"fmt"
	"math"
)

type UpdateStrategy interface {
	// Move the particle "in place" by writing to it scratch.
	MoveParticle(par *Particle, informer Particle)
}

type StandardUpdateStrategy struct {
}

func (us StandardUpdateStrategy) MoveParticle(par *Particle, informer Particle) {
	adapt := 0.99
	soc := 2.05 * math.Pow(adapt, float64(informer.BestAge))
	cog := 2.05 * math.Pow(adapt, float64(par.BestAge))
	momentum := 0.6
	dims := len(par.Pos)

	rand_soc := NewUniformVecFloat64(dims).SMul(cog)
	rand_cog := NewUniformVecFloat64(dims).SMul(soc)

	to_personal := par.BestPos.Sub(par.Pos)
	to_informer := informer.BestPos.Sub(par.Pos)
	fmt.Println("to_p, to_i:", to_personal, to_informer)

	(&to_personal).MulBy(rand_cog).SMulBy(cog)
	(&to_informer).MulBy(rand_soc).SMulBy(soc)
	fmt.Println("-> to_p, to_i:", to_personal, to_informer)

	acc := to_personal.Add(to_informer)
	vel := par.Vel.SMul(momentum).Add(acc)
	pos := par.Pos.Add(vel)

	// Set velocity and position directly in scratch area.
	par.TmpVel.Replace(vel)
	par.TmpPos.Replace(pos)
}
