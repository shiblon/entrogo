package main

import (
	//	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/shiblon/entrogo/fitness"
	"github.com/shiblon/entrogo/pso"
	"github.com/shiblon/entrogo/pso/topology"
)

// ./main -fit=rosenbrock:100:0.25 -topo=star:5 -m0=0.75 -m1=0.4 -cdecay=0.999 -mtype=randexplore -n=250000

var (
	fitnessFlag = flag.String("fit", "parabola:100:0.25",
		"Name of the fitness function. Specify parameters thus: "+
			"--fit=rastrigin:100:0.25 (for 100 dimensions, "+
			"and an offset of 1/4 each domain side length).")

	topoFlag = flag.String("topo", "star:5",
		"Name of the topology. Specify parameters thus: --topo=ring:3 or --topo=expander:6:2")

	iterFlag = flag.Int("n", 250000, "Number of evaluations.")

	outFreqFlag = flag.Int("outputfreq", 25000, "Evaluations between outputs.")

	outAllFlag = flag.Bool("outputall", false, "Output all particles instead of just the best.")

	m0Flag = flag.Float64("m0", 0.75, "'Starting' momentum.")
	m1Flag = flag.Float64("m1", 0.4, "'Ending' momentum.")

	radiusMultiplierFlag = flag.Float64("rmul", 0.1, "Fraction of domain to use as an initial radius.")
	radiusDecayFlag      = flag.Float64("rdecay", 0.9, "Decay rate of radius (and inverse bounce).")
	cognitiveDecayFlag   = flag.Float64("cdecay", 0.999, "Decay rate of cognitive constants.")
	momentumDecayFlag    = flag.Float64("mdecay", 0.9, "Decay rate for momentum calculations.")
	momentumTypeFlag     = flag.String("mtype", "linear", "Type of momentum.")
	tugTypeFlag          = flag.String("ttype", "none", "Type of 'tug', which how momentum is altered based on its direction as compared with the acceleration computation.")
	socConstFlag         = flag.Float64("sc", 2.05, "Social constant")
	cogConstFlag         = flag.Float64("cc", 2.05, "Cognitive constant")
	socLowerFlag         = flag.Float64("sclb", 0.0, "Social constant lower bound.")
	cogLowerFlag         = flag.Float64("cclb", 0.0, "Cognitive constant lower bound.")
)

var sflagre = regexp.MustCompile(`^\s*(\w+)(?::(.*))?\s*$`)

func parseStringFlag(str string) (name string, args []string) {
	match := sflagre.FindStringSubmatch(str)
	if match == nil {
		panic(fmt.Sprintf("Failed to parse flag '%s'", str))
	}
	name = match[1]
	args = strings.Split(match[2], ":")
	return name, args
}

func parseInt(val string) int {
	if intval, err := strconv.Atoi(val); err != nil {
		panic(fmt.Sprintf("Failed to parse '%v' to int: %v", val, err))
	} else {
		return intval
	}
}

func parseFloat(val string) float64 {
	if floatval, err := strconv.ParseFloat(val, 64); err != nil {
		panic(fmt.Sprintf("Failed to parse '%v' to float: %v", val, err))
	} else {
		return floatval
	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UTC().UnixNano())

	flag.Parse()

	fitname, fitargs := parseStringFlag(*fitnessFlag)
	toponame, topoargs := parseStringFlag(*topoFlag)

	var fitfunc fitness.Function
	switch fitname {
	case "parabola", "sphere":
		fitfunc = fitness.NewParabola(parseInt(fitargs[0]), parseFloat(fitargs[1]))
	case "rastrigin":
		fitfunc = fitness.NewRastrigin(parseInt(fitargs[0]), parseFloat(fitargs[1]))
	case "rosenbrock":
		fitfunc = fitness.NewRosenbrock(parseInt(fitargs[0]), parseFloat(fitargs[1]))
	case "ackley":
		fitfunc = fitness.NewAckley(parseInt(fitargs[0]), parseFloat(fitargs[1]))
	case "easom":
		fitfunc = fitness.NewEasom(parseInt(fitargs[0]), parseFloat(fitargs[1]))
	case "schwefel":
		fitfunc = fitness.NewSchwefel(parseInt(fitargs[0]), parseFloat(fitargs[1]))
	case "dejongf4":
		fitfunc = fitness.NewDeJongF4(parseInt(fitargs[0]), parseFloat(fitargs[1]))
	default:
		panic(fmt.Sprintf("Unknown function name '%v' in flag '%v'.", fitname, *fitnessFlag))
	}

	/*
		// Hough fitness
		circleStddevMultiplier := 0.2
		numCircles := 1
		img, houghFeatures := fitness.FeaturesFromImageFile("circles--magnitude.png")

		fitfuncs["houghcircle"] = fitness.NewHoughCircle(
			img.Bounds(),
			houghFeatures,
			numCircles,
			circleStddevMultiplier)

		// fitfuncs["houghtemplate"] = MakeHoughTemplateFunc()
	*/

	var topo topology.Topology
	switch toponame {
	case "ring":
		topo = topology.NewRing(parseInt(topoargs[0]))
	case "star":
		topo = topology.NewStar(parseInt(topoargs[0]))
	case "expander":
		topo = topology.NewRandomExpander(rand.NewSource(rand.Int63()), parseInt(topoargs[0]), parseInt(topoargs[1]))
	default:
		panic(fmt.Sprintf("Unknown topology name '%v' in flag '%v'.", toponame, *topoFlag))
	}

	outputevery := *outFreqFlag

	numIters := *iterFlag

	config := pso.NewBasicConfig(func() rand.Source {
		return rand.NewSource(rand.Int63())
	})
	config.DecayAdapt = *cognitiveDecayFlag
	config.DecayRadius = *radiusDecayFlag
	config.RadiusMultiplier = *radiusMultiplierFlag
	config.Momentum0 = *m0Flag
	config.Momentum1 = *m1Flag
	config.SocConst = *socConstFlag
	config.CogConst = *cogConstFlag
	config.SocLower = *socLowerFlag
	config.CogLower = *cogLowerFlag

	switch *tugTypeFlag {
	case "none":
		// Use the default tug function.
	case "dtrunc":
		// Truncate to 0 if < 0
		config.Tug = func(dot float64) float64 {
			if dot < 0 {
				return 0.0
			}
			return 1.0
		}
	case "rtrunc":
		// Truncate to 0 if < 0, based on a weighted flip.
		randChan := make(chan float64, 1)
		go func() {
			for {
				randChan <- rand.Float64()
			}
		}()
		config.Tug = func(dot float64) float64 {
			if dot < 0 && <-randChan <= math.Abs(dot) {
				return 0.0
			}
			return 1.0
		}
	case "dflip":
		// Deterministic flip - if dot product < 0, send -1.0.
		config.Tug = func(dot float64) float64 {
			if dot < 0 {
				return -1.0
			}
			return 1.0
		}
	case "rflip":
		// Random flip - if dot < 0, use -dot as the probability of a flip.
		randChan := make(chan float64, 1)
		go func() {
			for {
				randChan <- rand.Float64()
			}
		}()
		config.Tug = func(dot float64) float64 {
			if dot < 0 && <-randChan <= -dot {
				return -1.0
			}
			return 1.0
		}
	case "rdflip":
		// Flip in either direction based on probabilities.
		// If the dot product is close to -1 or 1, then it should be less probable to flip it.
		randChan := make(chan float64, 1)
		go func() {
			for {
				randChan <- rand.Float64()
			}
		}()
		config.Tug = func(dot float64) float64 {
			if <-randChan > math.Abs(dot) {
				return -dot
			}
			return dot
		}
	case "dweight":
		// Just multiply the momentum by the dot product.
		config.Tug = func(dot float64) float64 {
			return dot
		}
	default:
		panic(fmt.Sprintf("Unknown tug type: %s", *tugTypeFlag))
	}

	switch *momentumTypeFlag {
	case "constant":
		// Use the default momentum function.
	case "linear":
		config.Momentum = func(u pso.Updater, iter int, particle int) float64 {
			if iter >= numIters {
				iter = numIters - 1
			}
			factor := float64(iter) / float64(numIters)
			return (1-factor)*config.Momentum0 + factor*config.Momentum1
		}
	case "randexplore":
		type stateType struct {
			improvements      int
			batches           int
			improvementWeight float64
			smoothedMomentum  float64
			momentum          float64
		}

		stateChan := make(chan stateType, 1)
		stateChan <- stateType{
			momentum:         config.Momentum0,
			smoothedMomentum: config.Momentum0,
		}
		// Start supplying random values in a thread-safe manner.
		randMomentum := make(chan float64)
		go func() {
			for {
				randMomentum <- rand.Float64()*(config.Momentum1-config.Momentum0) + config.Momentum0
			}
		}()
		improvementDecay := *momentumDecayFlag
		config.Momentum = func(u pso.Updater, iter int, particle int) float64 {
			state := <-stateChan
			defer func() { stateChan <- state }()
			imp, bat := u.Batches()
			defer func() { state.batches, state.improvements = bat, imp }()
			if bat == state.batches {
				return state.momentum
			}
			state.improvementWeight *= improvementDecay
			if imp != state.improvements {
				// New batch, and we improved last time.
				state.improvementWeight += 1.0
				state.smoothedMomentum += (1.0 - improvementDecay) * (state.momentum - state.smoothedMomentum)
			}
			// We always calculate the improvement factor and use that to
			// determine the next momentum value, weighing things between
			// exploring randomly and exploiting the smoothed (only on
			// improvements) value.
			impFactor := (1 - improvementDecay) * state.improvementWeight // normalize to [0, 1]
			state.momentum = state.smoothedMomentum + (1.0-impFactor)*(<-randMomentum-state.smoothedMomentum)
			return state.momentum
		}
	case "randexplore2":
		type stateType struct {
			improvements        int
			batches             int
			smoothedImprovement float64
			smoothedMomentum    float64
			momentum            float64
		}

		stateChan := make(chan stateType, 1)
		stateChan <- stateType{
			momentum:         config.Momentum0,
			smoothedMomentum: config.Momentum0,
		}
		// Start supplying random values in a thread-safe manner.
		randMomentum := make(chan float64)
		go func() {
			for {
				randMomentum <- rand.Float64()*(config.Momentum1-config.Momentum0) + config.Momentum0
			}
		}()
		improvementDecay := *momentumDecayFlag
		config.Momentum = func(u pso.Updater, iter int, particle int) float64 {
			state := <-stateChan
			defer func() { stateChan <- state }()
			imp, bat := u.Batches()
			defer func() { state.batches, state.improvements = bat, imp }()
			if bat == state.batches {
				return state.momentum
			}
			improved := 0.0
			if imp != state.improvements {
				improved = 1.0
				state.smoothedMomentum += (1.0 - improvementDecay) * (state.momentum - state.smoothedMomentum)
			}
			state.smoothedImprovement += (1.0 - improvementDecay) * (improved - state.smoothedImprovement)
			// We always calculate the improvement factor and use that to
			// determine the next momentum value, weighing things between
			// exploring randomly and exploiting the smoothed (only on
			// improvements) value.
			state.momentum = state.smoothedMomentum + (1.0-state.smoothedImprovement)*(<-randMomentum-state.smoothedMomentum)
			return state.momentum
		}
	case "prandexplore":
		// per-particle random momentum exploration
		type particleState struct {
			t                   int
			smoothedImprovement float64
			smoothedMomentum    float64
			momentum            float64
		}
		stateChan := make(chan map[int]particleState, 1)
		stateChan <- make(map[int]particleState)
		randMomentum := make(chan float64)
		go func() {
			for {
				randMomentum <- rand.Float64()*(config.Momentum1-config.Momentum0) + config.Momentum0
			}
		}()
		improvementDecay := *momentumDecayFlag
		config.Momentum = func(u pso.Updater, iter int, pidx int) float64 {
			state := <-stateChan
			defer func() { stateChan <- state }()
			particle := u.Swarm()[pidx]
			pstate, ok := state[pidx]
			if !ok {
				pstate = particleState{
					t:                particle.T,
					smoothedMomentum: config.Momentum0,
					momentum:         config.Momentum0,
				}
				state[pidx] = pstate
			}
			defer func() { pstate.t = particle.T }()
			// Calculate new values - we have incremented time.
			if pstate.t != particle.T {
				improved := 0.0
				if particle.BestT == particle.T { // improved last time
					improved = 1.0
					pstate.smoothedMomentum += (1.0 - improvementDecay) * (pstate.momentum - pstate.smoothedMomentum)
				}
				pstate.smoothedImprovement += (1.0 - improvementDecay) * (improved - pstate.smoothedImprovement)
				pstate.momentum = pstate.smoothedMomentum + (1.0-pstate.smoothedImprovement)*(<-randMomentum-pstate.smoothedMomentum)
			}
			return pstate.momentum
		}
	case "recencyexplore":
		type stateType struct {
			t            int
			bestT        int
			improvements int
			momentum     float64
		}
		stateChan := make(chan stateType, 1)
		stateChan <- stateType{momentum: config.Momentum0}
		// Start supplying random values in a thread-safe manner.
		randMomentum := make(chan float64)
		go func() {
			for {
				randMomentum <- rand.Float64()*(config.Momentum1-config.Momentum0) + config.Momentum0
			}
		}()
		improvementDecay := *momentumDecayFlag
		config.Momentum = func(u pso.Updater, iter int, particle int) float64 {
			state := <-stateChan
			defer func() { stateChan <- state }()
			imp, bat := u.Batches()
			if bat != state.t {
				if imp != state.improvements {
					// The number of batch improvements changed, so we reset bestT.
					state.bestT = bat
				}
				state.t = bat
				state.improvements = imp

				factor := math.Pow(improvementDecay, float64(state.t-state.bestT))
				state.momentum = factor*state.momentum + (1-factor)*<-randMomentum
			}
			return state.momentum
		}
	case "precencyexplore":
		type particleState struct {
			t        int
			momentum float64
		}
		stateChan := make(chan map[int]particleState, 1)
		stateChan <- make(map[int]particleState)
		randMomentum := make(chan float64)
		go func() {
			for {
				randMomentum <- rand.Float64()*(config.Momentum1-config.Momentum0) + config.Momentum0
			}
		}()
		improvementDecay := *momentumDecayFlag
		config.Momentum = func(u pso.Updater, iter int, pidx int) float64 {
			state := <-stateChan
			defer func() { stateChan <- state }()
			particle := u.Swarm()[pidx]
			pstate, ok := state[pidx]
			if !ok {
				pstate = particleState{
					t:        particle.T,
					momentum: config.Momentum0,
				}
				state[pidx] = pstate
			}
			// Calculate new values if we have incremented time.
			if pstate.t != particle.T {
				pstate.t = particle.T
				factor := math.Pow(improvementDecay, float64(particle.T-particle.BestT))
				pstate.momentum = factor*pstate.momentum + (1-factor)*<-randMomentum
			}
			return pstate.momentum
		}
	case "histweight":
		state := struct {
			improvements      int
			batches           int
			improvementWeight float64
			momentum          float64
		}{
			momentum: config.Momentum0,
		}
		improvementDecay := *momentumDecayFlag
		config.Momentum = func(u pso.Updater, iter int, particle int) float64 {
			imp, bat := u.Batches()
			defer func() { state.batches, state.improvements = bat, imp }()
			if bat != state.batches {
				// New batch. Calculate new values.
				state.improvementWeight *= improvementDecay
				if imp != state.improvements {
					// New batch, and we improved last time.
					state.improvementWeight += 1.0
				}
				// The improvement factor is used to select between M0 and M1. If
				// lots of improvement is happening, we favor M0, otherwise we push
				// toward M1.
				impFactor := (1 - improvementDecay) * state.improvementWeight // normalize to [0, 1]
				state.momentum = impFactor*config.Momentum0 + (1.0-impFactor)*config.Momentum1
			}
			return state.momentum
		}
	}

	updater := pso.NewStandardPSO(topo, fitfunc, config)

	outputBest := func(evals int) {
		best := updater.BestParticle()
		fmt.Println(evals, "evals")
		fmt.Println(best, "momentum:", config.Momentum(updater, evals, best.Id))
		/*
			if soutput, err := json.Marshal(updater.Swarm()); err == nil {
				fmt.Printf("%d: %v\n", evals, string(soutput))
			} else {
				fmt.Errorf("Error converting to json: %v", err)
			}
		*/
	}

	outputAll := func(evals int) {
		fmt.Println(evals, "evals")
		for i, p := range updater.Swarm() {
			fmt.Println(i, p, "momentum:", config.Momentum(updater, evals, p.Id))
		}
	}

	outFn := outputBest
	if *outAllFlag {
		outFn = outputAll
	}

	evals := updater.Update()
	nextOutput := 0
	outFn(evals)
	for evals < *iterFlag {
		evals += updater.Update()
		if evals >= nextOutput {
			nextOutput += outputevery
			outFn(evals)
		}
	}
	outFn(evals)
}
