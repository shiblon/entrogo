package main

import (
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"math/rand"
	"monson/pso"
	"monson/pso/fitness"
	"monson/pso/topology"
	"monson/vec"
	"os"
	"runtime"
	"time"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UTC().UnixNano())

	numParticles := 2

	iterations := 1000000
	outputevery := 5000

	config := pso.NewBasicConfig()
	config.Momentum = 0.8
	config.DecayRadius = 0.99
	config.DecayAdapt = 0.999
	config.RadiusMultiplier = 0.1

	circleStddevMultiplier := 0.5

	// dims := 100
	// center := vec.NewFilled(dims, 9.0)
	// fitfunc := fitness.Parabola{Dims: dims, Center: center}
	// fitfunc := fitness.Rastrigin{Dims: dims, Center: center}

	// Hough fitness
	imgFile, err := os.Open("circle-mag.png")
	if err != nil {
		panic(fmt.Sprintf("Failed to open circle-mag.png: %v", err))
	}
	img, _, err := image.Decode(imgFile)
	if err != nil {
		panic(fmt.Sprintf("Failed to decode image: %v", err))
	}
	// Make feature list.
	// TODO: make a largish capacity list and fill it full of feature coordinates and magnitudes.
	features := make([]fitness.HoughPointFeature, 0, 1000)
	b := img.Bounds()
	fmt.Printf("bounds: %#v\n", b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.At(x, y)
			mag := c.(color.Gray).Y
			if mag > 0 {
				features = append(features, fitness.HoughPointFeature{float64(x), float64(y), float64(mag)})
			}
		}
	}
	fitfunc := fitness.NewHoughCircle(img.Bounds(), features, circleStddevMultiplier)
	// Redefine center to have the right size.
	center := vec.New(fitfunc.DomainDims())

	// topology := topology.NewRing(numParticles)
	topology := topology.NewStar(numParticles)

	updater := pso.NewStandardPSO(topology, fitfunc, config)

	// Increment by numParticles because that's how many evaluations we have done.
	for evals := 0; evals < iterations; {
		last := evals
		evals += updater.Update()
		if last/outputevery < evals/outputevery {
			best := updater.BestParticle()
			fmt.Println(best)
			dist_to_center := best.BestPos.Sub(center).Mag()
			fmt.Println(evals, best.BestVal, dist_to_center)
		}
	}
	best := updater.BestParticle()
	fmt.Println(best)
	fmt.Println(best.BestVal, best.BestPos.Sub(center).Mag())
}
