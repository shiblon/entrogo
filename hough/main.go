package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"log"
	"math"
	"os"
)

var (
	edgeFile = flag.String("image", "", "Input edge image (pre-processed).")
)

// Either it is there or it isn't. Simple first.
type FeatureSpace map[image.Point]bool

type ConvolutionImage struct {
	Width int
	Height int
	Pix [][]int
	MinVal int
	MaxVal int
}

func NewConvolutionImage(w, h int) (img ConvolutionImage) {
	img = ConvolutionImage{w, h, make([][]int, h), 0, 0}
	for y := 0; y < h; y++ {
		img.Pix[y] = make([]int, w)
	}
	return
}

func (img *ConvolutionImage) ToGray() (out *image.Gray) {
	out = image.NewGray(image.Rect(0, 0, img.Width, img.Height))
	dynamic_range := float64(img.MaxVal - img.MinVal + 1)
	start := 0
	for y := 0; y < img.Height; y++ {
		row := img.Pix[y]
		for x := 0; x < img.Width; x++ {
			// Map all values into range [0,255]
			val_weight := float64(row[x] - img.MinVal) / dynamic_range
			out.Pix[start + x] = uint8(val_weight * 255)
		}
		start += img.Width
	}
	return
}

func ToGray(img image.Image) (gimg *image.Gray) {
	gimg = image.NewGray(img.Bounds())
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			gimg.Set(x, y, color.GrayModel.Convert(img.At(x, y)))
		}
	}
	return
}

func PixelMergeConvolutionImages(images []ConvolutionImage, merger func([]int) int) (out ConvolutionImage) {
	w := images[0].Width
	h := images[0].Height

	for i := range images {
		if images[i].Width != w || images[i].Height != h {
			log.Fatalf("Inconsistent image sizes: expected %v,%v got %v,%v", w, h, images[i].Width, images[i].Height)
		}
	}

	out = NewConvolutionImage(w, h)

	value_list := make([]int, len(images))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			for i := range images {
				value_list[i] = images[i].Pix[y][x]
			}
			val := merger(value_list)
			if out.MaxVal < val {
				out.MaxVal = val
			}
			if out.MinVal > val {
				out.MinVal = val
			}
			out.Pix[y][x] = val
		}
	}
	return
}

func ConvolveGray(img *image.Gray, mtx [][]int8) (out ConvolutionImage) {
	b := img.Bounds()
	mh := len(mtx)
	mw := len(mtx[0])
	for i := 1; i < mh; i++ {
		if len(mtx[i]) != mw {
			log.Fatalf("Non-rectangular convolution matrix '%v'", mtx)
		}
	}

	out = NewConvolutionImage(b.Max.X-b.Min.X, b.Max.Y-b.Min.Y)

	for y := b.Min.Y; y < b.Max.Y-mh+1; y++ {
		for x := b.Min.X; x < b.Max.X-mw+1; x++ {
			val := 0
			for my := 0; my < mh; my++ {
				start := (y+my-img.Rect.Min.Y)*img.Stride + (x-img.Rect.Min.X)
				for mx := 0; mx < mw; mx++ {
					val += int(img.Pix[start+mx]) * int(mtx[my][mx])
				}
			}
			if out.MinVal > val {
				out.MinVal = val
			}
			if out.MaxVal < val {
				out.MaxVal = val
			}
			out.Pix[y-b.Min.Y][x-b.Min.X] = val
		}
	}
	return
}

func EdgeDetect(img *image.Gray) (ximg, yimg ConvolutionImage) {
	// TODO: we don't need the outer edge pixels, since we can't compute on those anyway.
	ximg = ConvolveGray(img, [][]int8{{1, 0, -1}, {2, 0, -2}, {1, 0, -1}})
	yimg = ConvolveGray(img, [][]int8{{1, 2, 1}, {0, 0, 0}, {-1, -2, -1}})
	return
}

func main() {
	flag.Parse()

	if *edgeFile == "" {
		log.Fatal("Must specify an image file.")
	}
	fmt.Printf("Reading image file '%v'\n", *edgeFile)
	imgfile, err := os.Open(*edgeFile)
	if err != nil {
		log.Fatalf("Failed to open '%v': '%v'", *edgeFile, err)
	}
	img, _, err := image.Decode(imgfile)

	// Convert to grayscale
	gimg := ToGray(img)

	// Detect edges
	xedge, yedge := EdgeDetect(gimg)
	magedge := PixelMergeConvolutionImages([]ConvolutionImage{xedge, yedge}, func(vals []int) int {
		s := 0.0
		for i := range (vals) {
			s += float64(vals[i]) * float64(vals[i])
		}
		return int(math.Sqrt(s))
	})
	angedge := PixelMergeConvolutionImages([]ConvolutionImage{xedge, yedge}, func(vals []int) int {
		return int(180*math.Atan2(float64(vals[1]), float64(vals[0]))/math.Pi)
	})

	xout, err := os.Create("xout.png")
	yout, err := os.Create("yout.png")
	mout, err := os.Create("mout.png")
	aout, err := os.Create("aout.png")

	png.Encode(xout, xedge.ToGray())
	png.Encode(yout, yedge.ToGray())
	png.Encode(mout, magedge.ToGray())
	png.Encode(aout, angedge.ToGray())

	fmt.Println("Hello world")
}
