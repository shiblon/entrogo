package vec

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

type NumArray2d struct {
	width, height int
	data          []float64
}

func NewNumArray2d(w, h int) *NumArray2d {
	return &NumArray2d{
		width:  w,
		height: h,
		data:   make([]float64, w*h),
	}
}

func NumArray2dFromGrayImage(img image.Gray) *NumArray2d {
	b := img.Bounds()
	out := NewNumArray2d(b.Max.X-b.Min.X, b.Max.Y-b.Min.Y)
	for i, v := range img.Pix {
		out.data[i] = float64(v)
	}
	return out
}

func (a *NumArray2d) Dimensions() (int, int) {
	return a.width, a.height
}

func (a *NumArray2d) Data() []float64 {
	return a.data
}

func (a *NumArray2d) Row(y int) []float64 {
	return a.data[y*a.width : (y+1)*a.width]
}

func (a *NumArray2d) At(x, y int) float64 {
	return a.data[y*a.width+x]
}

func (a *NumArray2d) AtExtended(x, y int) float64 {
	if x < 0 {
		x = 0
	} else if x >= a.width {
		x = a.width - 1
	}
	if y < 0 {
		y = 0
	} else if y >= a.height {
		y = a.height - 1
	}
	return a.At(x, y)
}

func (a *NumArray2d) AtWrapped(x, y int) float64 {
	return a.At(x%a.width, y%a.height)
}

func (a *NumArray2d) Set(x, y int, v float64) {
	a.data[y*a.width+x] = v
}

func (a *NumArray2d) Max() float64 {
	m := a.data[0]
	for _, v := range a.data {
		m = math.Max(m, v)
	}
	return m
}

func (a *NumArray2d) Min() float64 {
	m := a.data[0]
	for _, v := range a.data {
		m = math.Min(m, v)
	}
	return m
}

func (a *NumArray2d) Sum() float64 {
	s := 0.0
	for _, v := range a.data {
		s += v
	}
	return s
}

func (a *NumArray2d) Copy() *NumArray2d {
	n := NewNumArray2d(a.width, a.height)
	copy(n.data, a.data)
	return n
}

func (a *NumArray2d) Normalize() *NumArray2d {
	s := a.Sum()
	if s != 0.0 {
		for i, v := range a.data {
			a.data[i] = v / s
		}
	}
	return a
}

func (a *NumArray2d) Normalized() *NumArray2d {
	return a.Copy().Normalize()
}

func (a *NumArray2d) Standardize() *NumArray2d {
	mn := a.Min()
	mx := a.Max()
	dv := mx - mn
	for i, v := range a.data {
		a.data[i] = (v - mn) / dv
	}
	return a
}

func (a *NumArray2d) Standardized() *NumArray2d {
	return a.Copy().Standardize()
}

func (a *NumArray2d) Convolved(kernel *NumArray2d) (out *NumArray2d) {
	kw, kh := kernel.Dimensions()
	if kw%2 != 1 || kh%2 != 1 {
		panic(fmt.Sprintf("Width (%v) or height (%v) of kernel is not odd", kw, kh))
	}
	cx := (kw - 1) / 2
	cy := (kh - 1) / 2

	out = NewNumArray2d(a.width, a.height)

	for y := 0; y < a.height; y++ {
		for x := 0; x < a.width; x++ {
			// cx is at x, and cy is at y. Iterate over the convolution kernel
			// and get this new value.
			v := 0.0
			for ky := 0; ky < kh; ky++ {
				for kx := 0; kx < kw; kx++ {
					v += kernel.At(kx, ky) * a.AtExtended(x+kx-cx, y+ky-cy)
				}
			}
			out.Set(x, y, v)
		}
	}
	return
}

func (a *NumArray2d) ToGrayImage() (out *image.Gray) {
	out = image.NewGray(image.Rect(0, 0, a.width, a.height))
	a = a.Standardized()
	for y := 0; y < a.height; y++ {
		row := a.Row(y)
		newrow := out.Pix[y*a.width : (y+1)*a.width]
		for x := 0; x < a.width; x++ {
			newrow[x] = uint8(math.Floor(row[x] * 255.0))
		}
	}
	return out
}

func ImageToGray(img image.Image) (gimg image.Gray) {
	gimg = *image.NewGray(img.Bounds())
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			gimg.Set(x, y, color.GrayModel.Convert(img.At(x, y)))
		}
	}
	return
}

func (a *NumArray2d) MergeWith(b *NumArray2d, merger func(float64, float64) float64) *NumArray2d {
	if a.width != b.width || a.height != b.height {
		panic(fmt.Sprintf("Cannot merge different-sized images: w=%v,%v; h=%v,%v", a.width, b.width, a.height, b.height))
	}
	for i, v := range a.data {
		a.data[i] = merger(v, b.data[i])
	}
	return a
}

func (a *NumArray2d) MergedWith(b *NumArray2d, merger func(float64, float64) float64) *NumArray2d {
	return a.Copy().MergeWith(b, merger)
}

func (a *NumArray2d) Sobel2d() (xa, ya *NumArray2d) {
	xkernel := NewNumArray2d(3, 3)
	xkernel.data = []float64{-1, 0, 1, -2, 0, 2, -1, 0, 1}
	ykernel := NewNumArray2d(3, 3)
	ykernel.data = []float64{-1, -2, -1, 0, 0, 0, 1, 2, 1}

	xa = a.Convolved(xkernel)
	ya = a.Convolved(ykernel)
	return
}

func (a *NumArray2d) GnuplotOutput() {
	fmt.Println("# NumArray2d Output")
	for y := 0; y < a.height; y++ {
		for x := 0; x < a.width; x++ {
			fmt.Println(x, y, a.At(x, y))
		}
		fmt.Println()
	}
	fmt.Println()
}
