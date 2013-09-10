package pso

type Particle struct {
	Pos  VecFloat64
	Vel  VecFloat64
	Bpos VecFloat64
	Val  VecFloat64
	Bval VecFloat64
	Age  int32
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

	par.Bpos = make([]float64, dim)
	par.Bval = make([]float64, vdim)
}

func (par *Particle) UpdateBest() {
	par.Age = 0
	par.Bpos.Replace(par.Pos)
	par.Bval.Replace(par.Val)
}

type Swarm struct {
	Dim       int
	Vdim	  int
	Particles []*Particle
}

func NewSwarm(dim, vdim int, size int) (swarm *Swarm) {
	swarm = new(Swarm)
	swarm.Dim = dim
	swarm.Vdim = vdim
	swarm.Particles = make([]*Particle, size)

	for i := range swarm.Particles {
		swarm.Particles[i].Init(dim, vdim)
	}
	return
}
