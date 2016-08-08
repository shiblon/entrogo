package particle

import (
	"fmt"
	"math/rand"

	"github.com/shiblon/entrogo/vec"
	"github.com/shiblon/entrogo/fitness"
)

type TempParticleState struct {
	Pos, Vel vec.Vec
	Val      float64
	Bounced  bool
}

type Particle struct {
	Id int
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

	scratch *TempParticleState

	// Random number generator. Each particle gets its own.
	rgen *rand.Rand

	f fitness.Function
}

// NewRandomParticle gets its values by sampling from the fitness domain. It
// also evaluates the function if "evaluate" is true.
func NewRandomParticle(idx int, f fitness.Function) (par *Particle) {
	r := rand.New(rand.NewSource(rand.Int63()))
	pos := f.RandomPos(r)
	vel := f.RandomVel(r)
	return &Particle{
		Id:      idx,
		Pos:     pos,
		Vel:     vel,
		BestPos: pos.Copy(),
		scratch: &TempParticleState{
			Pos: pos.Copy(),
			Vel: vel.Copy(),
		},
		rgen: r,
		f:    f,
	}
}

func (p *Particle) ResetVal(val float64) {
	p.scratch.Val = val
	p.Val = val
	p.BestVal = val
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
	p.scratch.Pos = pos.Copy()
	p.scratch.Vel = vel.Copy()
	p.scratch.Val = val
}

func (p *Particle) Rand() *rand.Rand {
	return p.rgen
}

func (p *Particle) Scratch() *TempParticleState {
	return p.scratch
}

// Update the current state with the scratch state. This is useful if we are
// doing batch updates and need to compute other particle values based on a
// consistent time slice.
func (par *Particle) UpdateCur() {
	par.Pos.Replace(par.scratch.Pos)
	par.Vel.Replace(par.scratch.Vel)
	par.Val = par.scratch.Val
	if par.scratch.Bounced {
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

func (par *Particle) defaultInterpreter(v vec.Vec) string {
	return fmt.Sprintf("%f", v)
}

// Stringer
func (par *Particle) String() string {
	pos := par.f.VecInterpreter(par.Pos)
	vel := par.f.VecInterpreter(par.Vel)
	bpos := par.f.VecInterpreter(par.BestPos)
	return fmt.Sprintf(
		"Particle T=%d (%d):\n  f=%f x=%s\n  x'=%s\n  bf=%f bx=%s\n  bounces=%d",
		par.T, par.BestT, par.Val, pos, vel, par.BestVal, bpos, par.Bounces)
}
