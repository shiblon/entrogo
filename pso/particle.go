package pso

type Particle struct {
	// Current state
	Pos VecFloat64
	Vel VecFloat64
	Val VecFloat64

	// Current best state
	BestPos VecFloat64
	BestVal VecFloat64
	BestAge int32

	// Scratch space for delayed batch updates.
	TmpPos VecFloat64
	TmpVel VecFloat64
	TmpVal VecFloat64
}

func NewParticle(dim int, vdim int) (par *Particle) {
	par = new(Particle)
	par.Init(dim, vdim)
	return
}

func (par *Particle) Init(dim int, vdim int) {
	par.Pos = make([]float64, dim)
	par.Vel = make([]float64, dim)
	par.Val = make([]float64, vdim)

	par.BestPos = make([]float64, dim)
	par.BestVal = make([]float64, vdim)

	par.TmpPos = make([]float64, dim)
	par.TmpVel = make([]float64, dim)
	par.TmpVal = make([]float64, vdim)
}

// Update the current state with the scratch state. This is useful if we are
// doing batch updates and need to compute other particle values based on a
// consistent time slice.
func (par *Particle) UpdateCur() {
	par.Pos.Replace(par.TmpPos)
	par.Vel.Replace(par.TmpVel)
	par.Val.Replace(par.TmpVal)
	par.BestAge++
}

// We have determined that the current position is better than the current
// best. Overwrite the best and reset the best age.
func (par *Particle) UpdateBest() {
	par.BestAge = 0
	par.BestPos.Replace(par.Pos)
	par.BestVal.Replace(par.Val)
}
