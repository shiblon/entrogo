package pso

import (
	"fmt"
	"math/rand"
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
