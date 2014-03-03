package fitness

import (
	"code.google.com/p/entrogo/vec"
	"fmt"
	"image"
	"math"
	"math/rand"
)

type HoughCircle struct {
	basicFitness
	features []HoughPointFeature

	numCircles int

	width, height float64
	max_radius    float64

	domainDiameter float64
	bounds         image.Rectangle

	stddevFraction float64
}

func NewHoughCircle(bounds image.Rectangle, features []HoughPointFeature, numCircles int, stddevFraction float64) *HoughCircle {
	f := &HoughCircle{
		features:   features,
		bounds:     bounds,
		numCircles: numCircles,
	}
	oneMinCorner := []float64{float64(f.bounds.Min.X), float64(f.bounds.Min.Y), 2}
	oneMaxCorner := []float64{float64(f.bounds.Max.X), float64(f.bounds.Max.Y), f.max_radius}
	minCorner := make([]float64, 0, f.numCircles*3)
	maxCorner := make([]float64, 0, f.numCircles*3)
	for i := 0; i < f.numCircles; i++ {
		minCorner = append(minCorner, oneMinCorner...)
		maxCorner = append(maxCorner, oneMaxCorner...)
	}
	f.minCorner = vec.Vec(minCorner)
	f.maxCorner = vec.Vec(maxCorner)
	f.width = float64(bounds.Max.X - bounds.Min.X)
	f.height = float64(bounds.Max.Y - bounds.Min.Y)
	f.max_radius = math.Max(f.width, f.height)
	f.domainDiameter = math.Sqrt(f.width*f.width + f.height*f.height + f.max_radius*f.max_radius)
	f.stddevFraction = stddevFraction
	return f
}

func (f *HoughCircle) Query(pos vec.Vec) float64 {
	sum := 0.0
	// NOTE: This works! You can add stuff here and it will get ignored sanely.
	already_found := []vec.Vec{
	//{212.355591, 270.627908, 54.2781842},
	//{334.903469, 378.724900, 25.644862},
	//{478.330151, 267.377401, 76.516449},
	}
	// TODO: invert this so that the inner loop is over features, and then we
	// can properly pre-compute the log morginal.
	vals := vec.New(f.numCircles + len(already_found))
	for _, feature := range f.features {
		vals.Fill(0.0)
		for i := 0; i < f.numCircles; i++ {
			p := vec.Vec(pos[i*3 : (i+1)*3])
			vals[i] = f.oneCircleVoteForFeature(feature, p[0], p[1], p[2])
		}
		// Add the ones we already know about so that they are explained away.
		for i, p := range already_found {
			vals[f.numCircles+i] = f.oneCircleVoteForFeature(feature, p[0], p[1], p[2])
		}
		sum += math.Log(math.Max(0.001, vals.Max()))
	}
	return sum
}

func (f *HoughCircle) oneCircleVoteForFeature(feature HoughPointFeature, cx, cy, cr float64) float64 {
	multiplier := 1.0
	// Convert radius to something that has the range (0, +inf) and is reasonably linear in the feasible region.
	// NOTE: This is a place where you can make choices about the fitness function. Do you penalize bad values, or just make them impossible? We opt for the latter here.
	cr = math.Exp(cr / 20.0)
	// TODO: Transform these coordinates using a sigmoid, perhaps?
	// cx = Sigmoid(cx, 0, 1, f.width) + float64(f.bounds.Min.X)
	// cy = Sigmoid(cy, 0, 1, f.height) + float64(f.bounds.Min.Y)
	if cx < float64(f.bounds.Min.X) {
		multiplier /= 1 + float64(f.bounds.Min.X) - cx
	} else if cx > float64(f.bounds.Max.X) {
		multiplier /= 1 + cx - float64(f.bounds.Min.X)
	}
	if cy < float64(f.bounds.Min.Y) {
		multiplier /= 1 + float64(f.bounds.Min.Y) - cy
	} else if cy > float64(f.bounds.Max.Y) {
		multiplier /= 1 + cy - float64(f.bounds.Min.Y)
	}
	// Transform to unit circle space.
	x := (feature.X - cx) / cr
	y := (feature.Y - cy) / cr
	r := math.Sqrt(x*x + y*y)

	mu := 1.0
	stddev := f.stddevFraction
	// Note that we use cr again in normalization. This is because we already
	// moved the *domain* into normal circle space, but we haven't yet moved
	// the *range* into a sane place. If we don't normalize here, then every
	// point in space starts looking really good just because it has a big
	// radius and its normal circle is otherwise on par with every other normal
	// circle.
	norm := 1.0 / (2 * cr * math.Pi)

	// Normal distirbution
	// val := norm * Dnorm(r, mu, stddev)
	// Cauchy distribution
	val := norm * Dcauchy(r, mu, stddev)

	return multiplier * val
}

func (f *HoughCircle) Dims() int {
	return 3 * f.numCircles
}

func (f *HoughCircle) RandomPos(rgen *rand.Rand) vec.Vec {
	return UniformHyperrectSample(f.minCorner, f.maxCorner, rgen)
}

func (f *HoughCircle) RandomVel(rgen *rand.Rand) vec.Vec {
	return UniformHyperrectSample(f.maxCorner.Sub(f.minCorner).SDiv(2).Negated(), f.maxCorner.Sub(f.minCorner).SDiv(2), rgen)
}

func (f *HoughCircle) LessFit(a, b float64) bool {
	return a < b
}

func (f *HoughCircle) Diameter() float64 {
	return f.domainDiameter
}

func (f *HoughCircle) VecInterpreter(v vec.Vec) string {
	s := ""
	for i := 0; i < f.numCircles; i++ {
		this := v[i*3 : (i+1)*3]
		x, y, r := this[0], this[1], this[2]
		s += fmt.Sprintf("xy[%f %f] r:%f", x, y, r)
		if i < f.numCircles-1 {
			s += " | "
		}
	}
	return s
}
