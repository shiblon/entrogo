package fitness

import (
	"fmt"
	"image"
	"math"
	"math/rand"

	"github.com/shiblon/entrogo/vec"
)

const (
	OrientationSigma = 0.1
	DistanceSigma    = 0.0001
	ScaleSigma       = 0.01
	ScaleMax         = 3.0
)

type DiscreteTemplate struct {
	ox, oy, r float64
	w, h      int
	// We use a mental model of a pixel having its coordinates at its center, not its corner.
	features []HoughPointFeature
	// Holds indices, each into the feature nearest to this point. Ties are
	// broken by position in the feature list (take the first good on, don't
	// accept equally better ones later), which, in practical terms, means that
	// points more "up" and "left" are favored (lower numerical coordinate
	// values).
	nearest []int
}

// FindFeatureBounds returns information about how features are arranged. It
// finds a minimal boundin rectangle and a minimal bounding radius. The bounds
// returned are *exclusive*, meaning that if you were to draw pixels as
// rectangles, the bounds would be a thin line surrounding all of the area of
// the pixels. The mid point is calculated using these exclusive boundaries,
// and the radius of the minimal bounding circle is computed so that all of
// every feature's pixel area lies inside of the circle.
func FindFeatureBounds(features []HoughPointFeature) (xmin, ymin, xmax, ymax, radius float64) {
	f := features[0]
	xmin, xmax = f.X, f.X
	ymin, ymax = f.Y, f.Y
	for _, f = range features {
		xmin = math.Min(xmin, f.X)
		xmax = math.Max(xmax, f.X)
		ymin = math.Min(ymin, f.Y)
		ymax = math.Max(ymax, f.Y)
	}
	// we are not yet in the space where pixel coordinates fall at their
	// centers. Currently every coordinate is the upper left corner of a pixel.
	// Thus both min and max are *inclusive*. Here we fix that.
	xmax += 1
	ymax += 1

	// Now we find the center and the radius of the bounding circle.
	xmid := (xmin + xmax) / 2
	ymid := (ymin + ymax) / 2
	for _, f := range features {
		dx := f.X - xmid
		dy := f.Y - ymid
		// Pixels have width and are addressed by their corners. Adjust for this.
		if dx > 0 {
			dx += 1
		}
		if dy > 0 {
			dy += 1
		}
		radius = math.Max(radius, math.Hypot(dx, dy))
	}
	return
}

func NewDiscreteTemplate(features []HoughPointFeature) *DiscreteTemplate {
	xmin, ymin, xmax, ymax, radius := FindFeatureBounds(features)
	// With bounds on the features' actual coordinates, we can now normalize
	// those coordinates into a bounding box with the upper left at (0,0).
	w, h := int(xmax-xmin), int(ymax-ymin)
	out := &DiscreteTemplate{
		w:        w,
		h:        h,
		ox:       float64(w) / 2,
		oy:       float64(h) / 2,
		r:        radius,
		features: make([]HoughPointFeature, len(features)),
		nearest:  make([]int, w*h),
	}
	copy(out.features, features)
	// Now transform the features into center=(0,0) space. Pixel coordinates
	// now represent the center of the pixels. We make that happen by simply
	// adding .5 back to both coordinates after subtracting out the center.
	for i, f := range out.features {
		out.features[i].X, out.features[i].Y = out.ArrayToFeature(int(f.X-xmin), int(f.Y-ymin))
	}
	// Now all features are in center pixel coordinate space with the origin at
	// 0,0. The width and height are exclusive on both sides (draw a thin line
	// around the outside of the pixel grid) and the center is the middle of
	// that exclusive bounding box, with (0,0) being its upper-left corner.

	// There is a more efficient algorithm than this, to be sure, but this is a
	// one-time thing on a not-huge feature set, so we don't care.
	// Compute the nearest feature for each square in our grid.
	for y := 0; y < out.h; y++ {
		for x := 0; x < out.w; x++ {
			xfeat, yfeat := out.ArrayToFeature(x, y)
			// fmt.Println("# x", x, "xfeat", xfeat, "y", y, "yfeat", yfeat)
			// A straight Manhattan distance computation is sufficient for this.
			best_d := float64(out.w + out.h)
			best_i := 0
			for i, f := range out.features {
				d := math.Abs(f.X-xfeat) + math.Abs(f.Y-yfeat)
				// fmt.Printf("# dist: (%f %f) (%f %f) = %f\n", xfeat, yfeat, f.X, f.Y, d)
				if d < best_d {
					best_d = d
					best_i = i
				}
			}
			out.nearest[y*out.w+x] = best_i
		}
	}
	return out
}

func (t *DiscreteTemplate) Features() []HoughPointFeature {
	features := make([]HoughPointFeature, len(t.features))
	copy(features, t.features)
	for i, f := range t.features {
		x, y := t.FeatureToArray(f.X, f.Y)
		features[i].X, features[i].Y = float64(x), float64(y)
	}
	return features
}

func (t *DiscreteTemplate) String() string {
	s := "# Discrete Template\n"
	s += fmt.Sprintf("# w=%d, h=%d, r=%f, ox=%f, oy=%f\n", t.w, t.h, t.r, t.ox, t.oy)
	s += "# x y mag magnearest\n"
	for y := 0; y < t.h; y++ {
		for x := 0; x < t.w; x++ {
			near_index := t.nearest[y*t.w+x]
			near_feature := t.features[near_index]
			ax, ay := t.FeatureToArray(near_feature.X, near_feature.Y)
			mag := 0.0
			if ax == x && ay == y {
				mag = near_feature.Mag
			}
			near_mag := near_feature.Mag
			s += fmt.Sprintf("%d %d %f %f\n", x, y, mag, near_mag)
		}
		s += "\n"
	}
	s += "\n"
	return s
}

func (t *DiscreteTemplate) FeatureToArray(x, y float64) (int, int) {
	return int(x + t.ox - 0.5), int(y + t.oy - 0.5)
}

func (t *DiscreteTemplate) ArrayToFeature(x, y int) (float64, float64) {
	return float64(x) - t.ox + 0.5, float64(y) - t.oy + 0.5
}

// NearestFeature returns the feature that is closest to the requested position
// in feature coordinate space. Since we used manhattan distance to compute all
// of the nearest pixel information, we can just use that to find the nearest
// boundary pixel, as well. Manhattan distance gives the exact same ordering
// when the distance is greater than 1.0 away, so it's a fine appraoch to
// figuring out what's closest.
func (t *DiscreteTemplate) NearestFeature(x, y float64) HoughPointFeature {
	// Find the nearest pixel to us. If we are outside of the bounding box,
	// just clip until we are inside of it. That will provide us with the right
	// point to observe (closest boundary point by manhattan distance).
	//
	// Note that it's easiest to transform into array space immediately and do
	// all computations there for this step.
	ax, ay := t.FeatureToArray(x, y)

	if ax < 0 {
		ax = 0
	} else if ax >= t.w {
		ax = t.w - 1
	}
	if ay < 0 {
		ay = 0
	} else if ay >= t.h {
		ay = t.h - 1
	}
	best_i := t.nearest[ay*t.w+ax]
	return t.features[best_i]
}

func TemplateFromImageFile(fname string) (image.Gray, *DiscreteTemplate) {
	img, features := FeaturesFromImageFile(fname)
	return img, NewDiscreteTemplate(features)
}

type HoughTemplate struct {
	templates     map[string]*DiscreteTemplate
	templateNames []string // ordered templates
	features      []HoughPointFeature

	width, height float64
	max_radius    float64

	minCorner vec.Vec
	maxCorner vec.Vec

	domainDiameter float64
	bounds         image.Rectangle

	x_mu, x_sigma               float64
	y_mu, y_sigma               float64
	scale_mu, scale_sigma       float64
	template_mu, template_sigma float64
}

func NewHoughTemplate(bounds image.Rectangle, features []HoughPointFeature, templates map[string]*DiscreteTemplate) *HoughTemplate {
	f := &HoughTemplate{
		templates:      templates,
		features:       features,
		bounds:         bounds,
		width:          float64(bounds.Max.X - bounds.Min.X),
		height:         float64(bounds.Max.Y - bounds.Min.Y),
		x_mu:           float64(bounds.Max.X+bounds.Min.X) / 2.0,
		y_mu:           float64(bounds.Max.Y+bounds.Min.Y) / 2.0,
		x_sigma:        float64(bounds.Max.X-bounds.Min.X) / 4.0,
		y_sigma:        float64(bounds.Max.Y-bounds.Min.Y) / 20.0,
		scale_mu:       1.0,
		scale_sigma:    ScaleSigma,
		template_mu:    float64(len(templates) / 2),
		template_sigma: float64(len(templates)),
		minCorner:      vec.Vec([]float64{float64(bounds.Min.X), float64(bounds.Min.Y), 0.0, 0.0, 0.0}),
		maxCorner:      vec.Vec([]float64{float64(bounds.Max.X), float64(bounds.Max.Y), ScaleMax, ScaleMax, float64(len(templates) - 1)}),
	}
	// TODO: is there a sane way to order these so that there is some kind of a gradient?
	// Probably not....
	for k := range f.templates {
		f.templateNames = append(f.templateNames, k)
	}
	f.domainDiameter = f.maxCorner.Sub(f.minCorner).Mag()
	return f
}

func (f *HoughTemplate) Features() []HoughPointFeature {
	return f.features
}

func (f *HoughTemplate) ScanAtPos(x, y, scalemin, scalemax, step float64) {
	fmt.Println("# Func HoughTemplate scale scan START")
	v := vec.Vec{x, y, scalemin, scalemin, 0.0}
	for v[3] = scalemin; v[3] <= scalemax; v[3] += step {
		for v[2] = scalemin; v[2] <= scalemax; v[2] += step {
			fmt.Println(v[2], v[3], f.Query(v))
		}
		fmt.Println()
	}
	fmt.Println()
	fmt.Println("# Func HoughTemplate scale scan END")
}

func (f *HoughTemplate) ScanAtScale(sx, sy, xmin, ymin, xmax, ymax, step float64) {
	fmt.Println("# Func HoughTemplate pos scan START")
	v := vec.Vec{0.0, 0.0, sx, sy, 0.0}
	for v[1] = ymin; v[1] <= ymax; v[1] += step {
		for v[0] = xmin; v[0] <= xmax; v[0] += step {
			fmt.Println(v[0], v[1], f.Query(v))
		}
		fmt.Println()
	}
	fmt.Println()
	fmt.Println("# Func HoughTemplate pos scan END")
}

func (f *HoughTemplate) Query(pos vec.Vec) float64 {
	cx, cy, sx, sy, tindex := pos[0], pos[1], pos[2], pos[3], pos[4]

	// To compute the log of the probability over distance, we can do a lot of
	// very simple computations and sums before invoking the density function.
	// Because a log of a product of normal distributions becomes a
	// distribution over a sum, we can avoid a lot of computation and only take
	// logarithms of constants. To compute, for example, a product of
	// distributions over distances to each feature, we can do the following:
	//
	// sum_{i=1}^N max (log(min), -.5*log(2*pi) - log(sigma) - 1/(2*sigma^2) * d_i^2)
	//
	// Technically, that looks like this, but because we employ a minimum
	// probability *for each feature*, we have to make sure that the summation
	// is the outermost operator for each component probability.
	//
	// -(N/2)*log(2*pi*sigma^2) - sum_{i=1}^N(dist_i^2) / (2 * sigma^2)

	// Similarly, orientation on a truncated normal is computed thus:
	//
	// sum_{i=1}^N max (log(min), -.5*log(2*pi) - log(sigma*T) - 1/(2*sigma^2) * (cos t_i - 1)^2)
	// Where T is the truncated normal mass for the orientation computation.
	//
	// Combining these is trivial because it is just a couple of sums.

	// Note that because we are going to compute the sum of log likelihoods
	// below, we can actually just compute the sum of the log of these, as
	// well. The math works out far better that way.
	// TODO: There might be an issue with the mixture of a prior density
	// product with a likelihood mass. Should we be doing something with an integral here?
	s_dist := Fdtrunc2norm(0, ScaleMax, f.scale_mu, f.scale_sigma)
	x_dist := Fdtrunc2norm(0, f.width, f.x_mu, f.x_sigma)
	y_dist := Fdtrunc2norm(0, f.height, f.y_mu, f.y_sigma)
	log_prior := (math.Log(s_dist(sx)) +
		math.Log(s_dist(sy)) +
		math.Log(x_dist(cx)) +
		math.Log(y_dist(cy)) +
		math.Log(Dnorm(tindex, f.template_mu, f.template_sigma)))

	log_posterior := log_prior

	// For the truncated angle distribution, half of the mass is gone (this
	// stops at the peak at 1.0), and we are subtracting out the tail left of -1.

	N := float64(len(f.features))

	HalfLog2Pi := 0.5 * math.Log(2*math.Pi)
	MinLogProb := math.Log(MinProb)

	distance_sum := 0.0
	distance_scalar := -1.0 / (2 * DistanceSigma * DistanceSigma)
	distance_coefficient := -HalfLog2Pi - math.Log(DistanceSigma)
	// We'll compute (distance_coefficient + distance_scalar * dist*dist) at each iteration.

	T := 0.5 - Cnorm(-1.0, 1.0, OrientationSigma)
	angle_sum := 0.0
	angle_scalar := -1.0 / (2 * OrientationSigma * OrientationSigma)
	angle_coefficient := -HalfLog2Pi - math.Log(OrientationSigma) - math.Log(T)
	// We'll compute (angle_coefficient + angle_scalar * cos*cos) at each iteration

	_, template := f.TemplateForIndex(tindex)
	for _, feature := range f.features {
		x := (feature.X - cx) / sx
		y := (feature.Y - cy) / sy

		nearest := template.NearestFeature(x, y)
		dx := nearest.X - x
		dy := nearest.Y - y
		dsq := dx*dx + dy*dy

		distance_sum += math.Max(MinLogProb, distance_coefficient+distance_scalar*dsq)

		cos_factor := math.Cos(feature.Orientation-nearest.Orientation) - 1

		angle_sum += math.Max(MinLogProb, angle_coefficient+angle_scalar*cos_factor*cos_factor)
	}
	log_posterior += distance_sum + angle_sum
	return log_posterior - 0.5*N*(math.Log(sx)+math.Log(sy)) - float64(len(template.features))
}

func (f *HoughTemplate) TemplateForIndex(i float64) (string, *DiscreteTemplate) {
	index := int(i)
	if index < 0 {
		index = 0
	} else if index >= len(f.templateNames) {
		index = len(f.templateNames) - 1
	}
	name := f.templateNames[index]
	return name, f.templates[name]
}

func (f *HoughTemplate) RandomPos(rgen *rand.Rand) vec.Vec {
	// Independent draws in all initial dimensions.
	return UniformHyperrectSample(f.minCorner, f.maxCorner, rgen)
}

func (f *HoughTemplate) RandomVel(rgen *rand.Rand) vec.Vec {
	max := f.maxCorner.Sub(f.minCorner)
	return UniformHyperrectSample(f.minCorner.Sub(max), max, rgen)
}

func (f *HoughTemplate) LessFit(a, b float64) bool {
	return a < b
}

func (f *HoughTemplate) SideLengths() vec.Vec {
	return f.maxCorner.AbsSub(f.minCorner)
}

func (f *HoughTemplate) Diameter() float64 {
	return f.domainDiameter
}

func (f *HoughTemplate) Dims() int {
	return 5
}

func (f *HoughTemplate) VecInterpreter(v vec.Vec) string {
	tname, _ := f.TemplateForIndex(v[4])
	return fmt.Sprintf("x,y:[%f %f] s:[%f %f] t:%f=%s", v[0], v[1], v[2], v[3], v[4], tname)
}
