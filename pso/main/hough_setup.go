package main

import (
	"captcha/fitness"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
)

var (
	scan   = flag.Bool("scan", false, "Scan the fitness function and output gnuplot.")
	imgout = flag.Bool("imgout", false, "Output edge images.")
	sx     = flag.Float64("sx", 1.0, "Scan at the given scale.")
	sy     = flag.Float64("sy", 1.0, "Scan at the given scale.")
)

func MakeHoughTemplateFunc() fitness.Function {
	// Hough template (for characters)
	templateNames := []string{
		// "3.png",
		"a.png",
		"b.png",
		// "cap-a.png",
		// "cap-d.png",
		"cap-h.png",
		// "cap-k.png",
		// "cap-v.png",
	}
	templates := make(map[string]*fitness.DiscreteTemplate)
	var img image.Gray
	for _, n := range templateNames {
		img, templates[n] = fitness.TemplateFromImageFile(n)
		if *imgout {
			gray_name := "gray--" + n
			mag_name := "mag--" + n
			ang_name := "ang--" + n
			if out, err := os.Create(gray_name); err == nil {
				png.Encode(out, image.Image(&img))
			} else {
				panic(fmt.Sprintf("Cannot create %s: %v", gray_name, err))
			}

			mag, ang := fitness.ImagesFromFeatures(templates[n])
			if out, err := os.Create(mag_name); err == nil {
				png.Encode(out, image.Image(mag))
			} else {
				panic(fmt.Sprintf("Cannot create %s: %v", mag_name, err))
			}
			if out, err := os.Create(ang_name); err == nil {
				png.Encode(out, image.Image(ang))
			} else {
				panic(fmt.Sprintf("Cannot create %s: %v", ang_name, err))
			}
		}
	}

	imgFilename := "../../../../../images/ma.png"
	// imgFilename := "../../../../../images/ma2.png"
	// imgFilename := "../../../../../images/tmobile2.jpeg"

	img, houghFeatures := fitness.FeaturesFromImageFile(imgFilename)
	fitfunc := fitness.NewHoughTemplate(img.Bounds(), houghFeatures, templates)
	if *imgout {
		base := filepath.Base(imgFilename)
		gray_name := "gray--" + base
		mag_name := "mag--" + base
		ang_name := "ang--" + base
		if out, err := os.Create(gray_name); err == nil {
			png.Encode(out, image.Image(&img))
		} else {
			panic(fmt.Sprintf("Cannot create %s: %v", gray_name, err))
		}

		mag, ang := fitness.ImagesFromFeatures(fitfunc)
		if out, err := os.Create(mag_name); err == nil {
			png.Encode(out, image.Image(mag))
		} else {
			panic(fmt.Sprintf("Cannot create %s: %v", mag_name, err))
		}
		if out, err := os.Create(ang_name); err == nil {
			png.Encode(out, image.Image(ang))
		} else {
			panic(fmt.Sprintf("Cannot create %s: %v", ang_name, err))
		}
		return nil
	}
	if *scan {
		b := img.Bounds()
		fitfunc.ScanAtScale(*sx, *sy, float64(b.Min.X), float64(b.Min.Y), float64(b.Max.X), float64(b.Max.Y), 1.0)
		fitfunc.ScanAtPos(39.0, 10.0, 0.2, 2.0, 0.01)
		return nil
	}
	return fitfunc
}
