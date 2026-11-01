// Package imagemanipulator resizes, crops, rotates and flips pictures, and
// encodes them again. It's plain Go, so it needs no plugin add, and runs the
// same on phones, on the web and on your server.
//
//	img, err := imagemanipulator.Decode(photo.Data)
//	img = imagemanipulator.Fit(img, 1200)
//	img = imagemanipulator.Rotate(img, 90)
//	jpeg, err := imagemanipulator.JPEG(img, 85)
package imagemanipulator

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif" // Decode reads these too
	"image/jpeg"
	"image/png"
	"math"
)

// Decode reads a JPEG, PNG or GIF. A GIF gives its first frame. It doesn't
// apply the orientation a camera writes into a JPEG, so a photo straight from
// a camera may need a Rotate.
//
//nimbus:param b the file's bytes
//nimbus:returns the picture
func Decode(b []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(b))
	return img, err
}

// JPEG encodes it. JPEG has no transparency: clear parts come out black.
//
//nimbus:param img the picture
//nimbus:param quality 1 to 100; 0 means 85
//nimbus:returns the file's bytes
func JPEG(img image.Image, quality int) ([]byte, error) {
	if quality <= 0 {
		quality = 85
	}
	var b bytes.Buffer
	err := jpeg.Encode(&b, img, &jpeg.Options{Quality: min(quality, 100)})
	return b.Bytes(), err
}

// PNG encodes it, keeping transparency.
//
//nimbus:param img the picture
//nimbus:returns the file's bytes
func PNG(img image.Image) ([]byte, error) {
	var b bytes.Buffer
	err := png.Encode(&b, img)
	return b.Bytes(), err
}

// rgba copies img to a known layout, with its top left at 0,0.
func rgba(img image.Image) *image.NRGBA {
	r := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(out, out.Bounds(), img, r.Min, draw.Src)
	return out
}

// Fit shrinks it so that neither side is over max, keeping its shape. A
// picture that's already small enough comes back as it is.
//
//nimbus:param img the picture
//nimbus:param max the longest either side may be, in pixels
//nimbus:returns the picture
func Fit(img image.Image, max int) image.Image {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if max <= 0 || (w <= max && h <= max) {
		return img
	}
	if w >= h {
		return Resize(img, max, int(math.Max(1, math.Round(float64(h)*float64(max)/float64(w)))))
	}
	return Resize(img, int(math.Max(1, math.Round(float64(w)*float64(max)/float64(h)))), max)
}

// Resize scales it to exactly w by h, averaging pixels on the way down and
// blending them on the way up.
//
//nimbus:param img the picture
//nimbus:param w the new width, in pixels
//nimbus:param h the new height, in pixels
//nimbus:returns the picture
func Resize(img image.Image, w, h int) image.Image {
	if w <= 0 || h <= 0 || img.Bounds().Empty() {
		return image.NewNRGBA(image.Rect(0, 0, max(w, 0), max(h, 0)))
	}
	src := rgba(img)
	return vertical(horizontal(src, w), h)
}

// weights are the source pixels behind one output pixel.
type weights struct {
	first int
	each  []float64
}

// plan works out how n pixels become m, along one side.
func plan(n, m int) []weights {
	out := make([]weights, m)
	scale := float64(n) / float64(m)
	// A filter at least one source pixel wide averages going down; going up
	// it's a blend of the two nearest.
	radius := math.Max(scale, 1) * 0.5
	if scale < 1 {
		radius = 1
	}
	for i := range out {
		center := (float64(i)+0.5)*scale - 0.5
		first := max(int(math.Ceil(center-radius)), 0)
		last := min(int(math.Floor(center+radius)), n-1)
		each := make([]float64, 0, last-first+1)
		total := 0.0
		for p := first; p <= last; p++ {
			w := 1.0
			if scale < 1 {
				w = 1 - math.Abs(float64(p)-center)
			}
			each = append(each, w)
			total += w
		}
		if total == 0 {
			first, each, total = min(int(math.Round(center)), n-1), []float64{1}, 1
		}
		for k := range each {
			each[k] /= total
		}
		out[i] = weights{first, each}
	}
	return out
}

// blend adds pixels up with alpha taken into account, so that clear pixels
// don't bleed their hidden color into the edges.
func blend(src []uint8, at, step int, w weights) color.NRGBA {
	var r, g, b, a float64
	for k, v := range w.each {
		p := src[at+(w.first+k)*step:]
		alpha := float64(p[3]) * v
		r += float64(p[0]) * alpha
		g += float64(p[1]) * alpha
		b += float64(p[2]) * alpha
		a += alpha
	}
	if a == 0 {
		return color.NRGBA{}
	}
	c := func(x float64) uint8 { return uint8(math.Min(math.Round(x), 255)) }
	return color.NRGBA{c(r / a), c(g / a), c(b / a), c(a)}
}

func horizontal(src *image.NRGBA, w int) *image.NRGBA {
	h := src.Bounds().Dy()
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	p := plan(src.Bounds().Dx(), w)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			out.SetNRGBA(x, y, blend(src.Pix, y*src.Stride, 4, p[x]))
		}
	}
	return out
}

func vertical(src *image.NRGBA, h int) *image.NRGBA {
	w := src.Bounds().Dx()
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	p := plan(src.Bounds().Dy(), h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			out.SetNRGBA(x, y, blend(src.Pix, x*4, src.Stride, p[y]))
		}
	}
	return out
}

// Crop cuts out part of it. Whatever falls outside the picture is left off.
//
//nimbus:param img the picture
//nimbus:param x the left edge, in pixels
//nimbus:param y the top edge, in pixels
//nimbus:param w the width to keep
//nimbus:param h the height to keep
//nimbus:returns the picture
func Crop(img image.Image, x, y, w, h int) image.Image {
	src := rgba(img)
	r := image.Rect(x, y, x+w, y+h).Intersect(src.Bounds())
	out := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(out, out.Bounds(), src, r.Min, draw.Src)
	return out
}

// Rotate turns it clockwise. Other angles than 90, 180 and 270 change nothing.
//
//nimbus:param img the picture
//nimbus:param degrees 90, 180 or 270
//nimbus:returns the picture
func Rotate(img image.Image, degrees int) image.Image {
	degrees = (degrees%360 + 360) % 360
	if degrees != 90 && degrees != 180 && degrees != 270 {
		return img
	}
	src := rgba(img)
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	bounds := image.Rect(0, 0, h, w)
	if degrees == 180 {
		bounds = image.Rect(0, 0, w, h)
	}
	out := image.NewNRGBA(bounds)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := src.NRGBAAt(x, y)
			switch degrees {
			case 90:
				out.SetNRGBA(h-1-y, x, c)
			case 180:
				out.SetNRGBA(w-1-x, h-1-y, c)
			default:
				out.SetNRGBA(y, w-1-x, c)
			}
		}
	}
	return out
}

// Flip mirrors it.
//
//nimbus:param img the picture
//nimbus:param vertical top to bottom; false is left to right
//nimbus:returns the picture
func Flip(img image.Image, vertical bool) image.Image {
	src := rgba(img)
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	out := image.NewNRGBA(src.Bounds())
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if vertical {
				out.SetNRGBA(x, h-1-y, src.NRGBAAt(x, y))
			} else {
				out.SetNRGBA(w-1-x, y, src.NRGBAAt(x, y))
			}
		}
	}
	return out
}
