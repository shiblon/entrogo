package swarm

import (
	"fmt"
	"math/rand"
	"monson/pso/fitness"
	"monson/vec"
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
	Rand *rand.Rand
}

// NewRandomParticle gets its values by sampling from the fitness domain. It
// also evaluates the function if "evaluate" is true.
func NewRandomParticle(f fitness.Function, evaluate bool) (par *Particle) {
	r := rand.New(rand.NewSource(rand.Int63()))
	pos := f.RandomPos(r)
	vel := f.RandomVel(r)
	val := 0.0
	if evaluate {
		val = f.Query(pos)
	}
	return &Particle{
		Pos: pos,
		Vel: vel,
		Val: val,
		BestPos: pos.Copy(),
		BestVal: val,
		TempPos: pos.Copy(),
		TempVel: vel.Copy(),
		TempVal: val,
		Rand: r,
	}
}

func (p *Particle) Init(pos, vel vec.Vec, val float64) {
	if len(pos) != len(vel) {
		panic(fmt.Sprintf("Position and velocity vecs have different lengths: %d != %d", len(pos), len(vel)))
	}
	p.Pos = pos.Copy()
	p.Vel = vel.Copy()
	p.Val = val
	p.BestPos = pos.Copy()
	p.BestVal = val
	p.BestAge = 0
	p.Bounces = 0
	p.TempPos = pos.Copy()
	p.TempVel = vel.Copy()
	p.TempVal = val
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
func (par *Particle) String() string {
	return fmt.Sprintf(
		"  x=%.3f  x'=%.3f\n  f=%.4f\n  bx=%.3f\n  bf=%.4f  ba=%v\n  bounces=%v",
		par.Pos, par.Vel, par.Val, par.BestPos, par.BestVal, par.BestAge, par.Bounces)
}
