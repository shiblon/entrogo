package main

import (
	"fmt"
	"math/rand"
	"monson/pso"
	"monson/pso/fitness"
	"monson/pso/topology"
	"monson/vec"
	"runtime"
	"time"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UTC().UnixNano())

	dims := 100
	iterations := 1000000
	numParticles := 2

	outputevery := 20000

	center := vec.NewFilled(dims, 9.0)

	// fitness := fitness.Parabola{Dims: dims, Center: center}
	fitness := fitness.Rastrigin{Dims: dims, Center: center}

	// topology := topology.NewRing(numParticles)
	topology := topology.NewStar(numParticles)
	updater := pso.NewStandardPSO(topology, fitness)

	// Increment by numParticles because that's how many evaluations we have done.
	for evals := 0; evals < iterations; {
		last := evals
		evals += updater.Update()
		if last/outputevery < evals/outputevery {
			best := updater.BestParticle()
			dist_to_center := best.BestPos.Sub(center).Mag()
			fmt.Println(evals, best.BestVal, dist_to_center)
		}
	}
	best := updater.BestParticle()
	fmt.Println(best)
	fmt.Println(best.BestVal, best.BestPos.Sub(center).Mag())
}
