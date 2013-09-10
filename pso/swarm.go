package pso

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
