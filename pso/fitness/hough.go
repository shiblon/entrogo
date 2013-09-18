package fitness

import (
	"math"
	"math/rand"
	"monson/vec"
)

type HoughPointFeature struct {
	X, Y float64
}

type HoughCircle struct {
	features []HoughPointFeature

	minx, miny, maxx, maxy float64

	domainDiameter float64
}

func NewHoughCircle(features []HoughPointFeature) *HoughCircle {
	c := &HoughCircle{features: features}
	c.minx = features[0].X
	c.miny = features[0].Y
	c.maxx = c.minx
	c.maxy = c.miny
	for _, feature := range features[1:] {
		switch {
		case c.minx > feature.X:
			c.minx = feature.X
		case c.maxx < feature.X:
			c.maxx = feature.X
		}
		switch {
		case c.minx > feature.Y:
			c.miny = feature.Y
		case c.maxy < feature.Y:
			c.maxy = feature.Y
		}
	}
	xd := c.maxx - c.minx
	yd := c.maxy - c.miny
	c.domainDiameter = math.Sqrt(float64(xd*xd + yd*yd))
	return c
}

func (f *HoughCircle) Query(pos vec.Vec) float64 {
	s := 0.0
	for _, feature := range f.features {
		s += f.voteForFeature(feature, pos)
	}
	return s
}

func (f *HoughCircle) voteForFeature(feature HoughPointFeature, pos vec.Vec) float64 {
	sx := pos[0]
	sy := pos[1]
	ax := pos[2]
	ay := pos[3]
	dx := pos[4]
	dy := pos[5]

	cx := feature.X * sx + feature.Y * ay + dx
	cy := feature.Y * sy + feature.X * ax + dy

	// The circle in this coordinate space is always centered on 0, so we don't
	// have to subtract to get distance from center. It also always has a radius of 1.0.
	d := math.Sqrt(cx*cx + cy*cy)
	mu := 1.0
	stdev := 0.05
	norm := 1.0 / (2 * math.Pi)

	return norm * math.Exp(-(d-mu)*(d-mu) / (2 * stdev*stdev))
}

func (f *HoughCircle) DomainDims() int {
	return 6 // TODO: is this right?
}

func (f *HoughCircle) RandomPos(rgen *rand.Rand) vec.Vec {
	return UniformCubeSample(f.DomainDims(), 0, f.RoughDomainDiameter(), rgen)
}

func (f *HoughCircle) RandomVel(rgen *rand.Rand) vec.Vec {
	return UniformCubeSample(f.DomainDims(), -f.RoughDomainDiameter(), f.RoughDomainDiameter(), rgen)
}

func (f *HoughCircle) LessFit(a, b float64) bool {
	return a < b
}

func (f *HoughCircle) RoughDomainDiameter() float64 {
	return f.domainDiameter
}
