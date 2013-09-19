package fitness

import (
	"fmt"
	"image"
	"math"
	"math/rand"
	"monson/vec"
)

// TODO: Remove when done debugging
var _p_ = fmt.Println

type HoughPointFeature struct {
	X, Y float64
	Mag float64
}

type HoughCircle struct {
	features []HoughPointFeature

	numCircles int

	width, height float64
	max_radius float64

	minCorner vec.Vec
	maxCorner vec.Vec

	domainDiameter float64
	bounds image.Rectangle

	stddevFraction float64
}

func NewHoughCircle(bounds image.Rectangle, features []HoughPointFeature, stddevFraction float64) *HoughCircle {
	f := &HoughCircle{
		features: features,
		bounds: bounds,
		numCircles: 1,
	}
	oneMinCorner := []float64{float64(f.bounds.Min.X), float64(f.bounds.Min.Y), 2}
	oneMaxCorner := []float64{float64(f.bounds.Max.X), float64(f.bounds.Max.Y), f.max_radius}
	minCorner := make([]float64, 0, f.numCircles * 3)
	maxCorner := make([]float64, 0, f.numCircles * 3)
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
	sums := vec.New(f.numCircles)
	for _, feature := range f.features {
		for i := 0; i < f.numCircles; i++ {
			cx := pos[i*3]
			cy := pos[i*3+1]
			r := pos[i*3+2]
			sums[i] += f.oneCircleVoteForFeature(feature, cx, cy, r)
		}
	}
	return sums.Sum()
}
func (f *HoughCircle) oneCircleVoteForFeature(feature HoughPointFeature, cx, cy, radius float64) float64 {
	x := feature.X - cx
	y := feature.Y - cy

	// The circle in this coordinate space is always centered on 0, so we don't
	// have to subtract to get distance from center. It also always has a radius of 1.0.
	d := math.Sqrt(x*x + y*y)
	mu := radius
	stddev := f.stddevFraction * radius
	norm := 1.0 / (2 * radius * math.Pi)

	// TODO: do we want to do something with intensity of edge magnitude?
	val := norm * math.Exp(-(d-mu)*(d-mu) / (2 * stddev*stddev))

	return val
}

func (f *HoughCircle) DomainDims() int {
	return 3 * f.numCircles
}

func (f *HoughCircle) RandomPos(rgen *rand.Rand) vec.Vec {
	return UniformHyperrectSample(f.minCorner, f.maxCorner, rgen)
}

func (f *HoughCircle) RandomVel(rgen *rand.Rand) vec.Vec {
	return UniformHyperrectSample(f.maxCorner.SDiv(2).Negate(), f.maxCorner.SDiv(2), rgen)
}

func (f *HoughCircle) LessFit(a, b float64) bool {
	return a < b
}

func (f *HoughCircle) RoughDomainDiameter() float64 {
	return f.domainDiameter
}
