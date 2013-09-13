package main

import (
	"fmt"
	"monson/pso"
)

func main() {
	topology := pso.RingTopology(7)
	updater := pso.StandardUpdateStrategy{}
	fitness := pso.FitnessParabola{Dims: 2, Center: pso.VecFloat64{0, 0}}

	swarm := pso.NewSwarm(topology, updater, fitness)
	fmt.Println(swarm)
	for i := 0; i < 100; i++ {
		swarm.BatchUpdate()
		fmt.Println(swarm)
	}
}
