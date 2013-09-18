package particle

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
	T        int // time

	// Current best state
	BestPos vec.Vec
	BestVal float64

	// Additional state
	BestT   int // time
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
		Pos:     pos,
		Vel:     vel,
		Val:     val,
		BestPos: pos.Copy(),
		BestVal: val,
		TempPos: pos.Copy(),
		TempVel: vel.Copy(),
		TempVal: val,
		Rand:    r,
	}
}

func (p *Particle) Init(pos, vel vec.Vec, val float64) {
	if len(pos) != len(vel) {
		panic(fmt.Sprintf("Position and velocity vecs have different lengths: %d != %d", len(pos), len(vel)))
	}
	p.Pos = pos.Copy()
	p.Vel = vel.Copy()
	p.Val = val
	p.T = 0
	p.BestPos = pos.Copy()
	p.BestVal = val
	p.BestT = 0
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
	par.T++
}

// We have determined that the current position is better than the current
// best. Overwrite the best and reset the best age.
func (par *Particle) UpdateBest() {
	par.BestPos.Replace(par.Pos)
	par.BestVal = par.Val
	par.BestT = par.T
}

// Stringer
func (par *Particle) String() string {
	return fmt.Sprintf(
		"%d (%d):\n  f=%.4f x=%.3f\n  x'=%.3f\n  bf=%.4f bx=%.3f\n  bounces=%d",
		par.T, par.BestT, par.Val, par.Pos, par.Vel, par.BestVal, par.BestPos, par.Bounces)
}
