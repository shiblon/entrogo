package pso

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

func NewParticle(dim int) (par *Particle) {
	par = new(Particle)
	par.Init(dim)
	return
}

func (par *Particle) Init(dim int) {
	par.Pos = make([]float64, dim)
	par.Vel = make([]float64, dim)
	par.Val = 0
	par.Age = 0

	par.BestPos = make([]float64, dim)
	par.BestVal = 0
	par.BestAge = 0

	par.TmpPos = make([]float64, dim)
	par.TmpVel = make([]float64, dim)
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
