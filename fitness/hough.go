package fitness

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"

	"github.com/shiblon/entrogo/vec"
)

type Featurer interface {
	Features() []HoughPointFeature
}

type HoughPointFeature struct {
	X, Y        float64
	Mag         float64
	Orientation float64 // [0,2pi) but any value is really possible.
}

// ImagesFromFeatures creates a grayscale image for magnitude and angle from Sobel features.
// It assumes that the coordinates, though floating point, are really representative of integers.
func ImagesFromFeatures(source Featurer) (mag, ang *image.Gray) {
	features := source.Features()
	minx, maxx, miny, maxy := features[0].X, features[0].X, features[0].Y, features[0].Y
	maxmag := 0.0
	for _, f := range features {
		minx = math.Min(minx, f.X)
		miny = math.Min(miny, f.Y)
		maxx = math.Max(maxx, f.X)
		maxy = math.Max(maxy, f.Y)
		maxmag = math.Max(maxmag, f.Mag)
	}
	bounds := image.Rect(
		int(math.Floor(minx)), int(math.Floor(miny)), int(math.Ceil(maxx)), int(math.Ceil(maxy)))
	mag = image.NewGray(bounds)
	ang = image.NewGray(bounds)

	for _, f := range features {
		x, y := int(f.X), int(f.Y)
		mag.Set(x, y, color.Gray{byte(255 * f.Mag / maxmag)})
		ang.Set(x, y, color.Gray{byte(1 + 254*f.Orientation/(2*math.Pi))})
	}
	return
}

func FeaturesFromImageFile(fname string) (image.Gray, []HoughPointFeature) {
	imgFile, err := os.Open(fname)
	if err != nil {
		panic(fmt.Sprintf("Failed to open %s: %v", fname, err))
	}
	img, _, err := image.Decode(imgFile)
	if err != nil {
		panic(fmt.Sprintf("Failed to decode image %s: %v", fname, err))
	}

	gimg := vec.ImageToGray(img)
	return gimg, SobelFeatures(gimg)
}

func SobelFeatures(img image.Gray) (features []HoughPointFeature) {
	threshold := 0.7
	arrayimg := vec.NumArray2dFromGrayImage(img).Standardize()
	w, h := arrayimg.Dimensions()
	xsobel, ysobel := arrayimg.Sobel2d()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx, sy := xsobel.At(x, y), ysobel.At(x, y)
			mag := math.Sqrt(sx*sx + sy*sy)
			if mag > threshold {
				fv := math.Atan2(sy, sx)
				features = append(features, HoughPointFeature{X: float64(x), Y: float64(y), Mag: mag, Orientation: fv})
			}
		}
	}
	return
}
