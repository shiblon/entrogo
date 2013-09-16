package pso

import (
	"fmt"
	"math/rand"
)

type Particle struct {
	// Current state
	Pos VecFloat64
	Vel VecFloat64
	Val float64
	Age int32

	// Current best state
	BestPos VecFloat64
	BestVal float64
	BestAge int32

	// Random number generator. Each particle gets its own.
	randGen *rand.Rand
}

func NewParticle(pos, vel VecFloat64, val float64, rgen *rand.Rand) (par *Particle) {
	if len(pos) != len(vel) {
		panic(fmt.Sprintf("Position and velocity vectors have different lengths: %d != %d", len(pos), len(vel)))
	}
	par = &Particle{}
	par.randGen = rgen
	par.Init(pos, vel, val)
	return
}

func (par *Particle) Init(pos, vel VecFloat64, val float64) {
	par.Pos = pos.Copy()
	par.Vel = vel.Copy()
	par.Val = val
	par.Age = 0

	par.BestPos = pos.Copy()
	par.BestVal = val
	par.BestAge = 0
}

// Update the current state with the scratch state. This is useful if we are
// doing batch updates and need to compute other particle values based on a
// consistent time slice.
func (par *Particle) UpdateCur(pos, vel VecFloat64, val float64) {
	par.Pos.Replace(pos)
	par.Vel.Replace(vel)
	par.Val = val
	par.BestAge++
	par.Age++
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
		"  x=%v  x'=%v\n  f=%v  a=%v\n  bx=%v\n  bf=%v  ba=%v",
		par.Pos, par.Vel, par.Val, par.Age, par.BestPos, par.BestVal, par.BestAge)
}
