package pso

type Swarm struct {
	Dim          int
	Vdim         int
	Particles    []*Particle
	Neighborhood func(int) []int
}

func NewSwarm(dim, vdim int, size int, neighborhood func(int) []int) (swarm *Swarm) {
	swarm = new(Swarm)
	swarm.Dim = dim
	swarm.Vdim = vdim
	swarm.Particles = make([]*Particle, size)
	swarm.Neighborhood = neighborhood

	for i := range swarm.Particles {
		swarm.Particles[i].Init(dim, vdim)
	}
	return
}
