package main

import (
	"fmt"
	"monson/pso"
	"monson/pso/fitness"
	"monson/pso/vec"
	"runtime"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	dims := 100
	iterations := 1000000
	seed := 1
	numParticles := 4

	center := vec.NewFilled(2.0, dims)

	// fitness := fitness.Parabola{Dims: dims, Center: pso.New(dims)}
	fitness := fitness.Rastrigin{Dims: dims, Center: center}

	// topology := pso.NewRingTopology(numParticles)
	topology := pso.NewStarTopology(numParticles)
	updater := pso.StandardUpdateStrategy{fitness.RoughDomainDiameter()}

	swarm := pso.NewSwarm(topology, updater, fitness, int64(seed))
	// Increment by numParticles because that's how many evaluations we have done.
	for i := numParticles; i < iterations; i += numParticles {
		last := i - numParticles
		if last%10000 == 0 || last/10000 < i/10000 {
			fmt.Println(swarm.BestParticle())
		}
		swarm.BatchUpdate()
	}
	fmt.Println(swarm.BestParticle())
}
