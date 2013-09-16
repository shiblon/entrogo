package main

import (
	"fmt"
	"monson/pso"
)

func main() {
	topology := pso.RingTopology(7)
	updater := pso.StandardUpdateStrategy{}
	fitness := pso.FitnessParabola{Dims: 100, Center: pso.NewZeroVecFloat64(100)}

	swarm := pso.NewSwarm(topology, updater, fitness)
	for i := 0; i < 100000; i++ {
		swarm.BatchUpdate()
	}
	fmt.Println(swarm)
}
