package pso

import "fmt"

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

	// Scratch space for delayed batch updates.
	TmpPos VecFloat64
	TmpVel VecFloat64
	TmpVal float64
}

func NewParticle(pos, vel VecFloat64) (par *Particle) {
	if len(pos) != len(vel) {
		panic(fmt.Sprintf("Position and velocity vectors have different lengths: %d != %d", len(pos), len(vel)))
	}
	par = &Particle{}
	par.Init(pos, vel)
	return
}

func (par *Particle) Init(pos, vel VecFloat64) {
	par.Pos = pos.Copy()
	par.Vel = vel.Copy()
	par.Val = 0
	par.Age = 0

	par.BestPos = pos.Copy()
	par.BestVal = 0
	par.BestAge = 0

	par.TmpPos = pos.Copy()
	par.TmpVel = vel.Copy()
	par.TmpVal = 0
}

// Update the current state with the scratch state. This is useful if we are
// doing batch updates and need to compute other particle values based on a
// consistent time slice.
func (par *Particle) UpdateCur() {
	par.Pos.Replace(par.TmpPos)
	par.Vel.Replace(par.TmpVel)
	par.Val = par.TmpVal
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
		"  x=%v  x'=%v\n  f=%v  a=%v\n  bx=%v\n  bf=%v  ba=%v\n  tx=%v  tx'=%v\n  tf=%v",
		par.Pos, par.Vel, par.Val, par.Age, par.BestPos, par.BestVal, par.BestAge, par.TmpPos,
		par.TmpVel, par.TmpVal)
}
