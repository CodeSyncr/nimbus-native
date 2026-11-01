package imagemanipulator

import (
	"image"
	"image/color"
	"testing"
)

var (
	red   = color.NRGBA{255, 0, 0, 255}
	blue  = color.NRGBA{0, 0, 255, 255}
	green = color.NRGBA{0, 255, 0, 255}
	white = color.NRGBA{255, 255, 255, 255}
)

// corners is two squares, s pixels each: red on the left, blue on the right.
func corners(s int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, s*2, s))
	for y := 0; y < s; y++ {
		for x := 0; x < s*2; x++ {
			c := red
			if x >= s {
				c = blue
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

func at(t *testing.T, img image.Image, x, y int, want color.NRGBA) {
	t.Helper()
	got := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	if got != want {
		t.Errorf("(%d,%d) = %v, want %v", x, y, got, want)
	}
}

func size(t *testing.T, img image.Image, w, h int) {
	t.Helper()
	if b := img.Bounds(); b.Dx() != w || b.Dy() != h {
		t.Fatalf("size = %dx%d, want %dx%d", b.Dx(), b.Dy(), w, h)
	}
}

func TestResize(t *testing.T) {
	small := Resize(corners(50), 20, 10)
	size(t, small, 20, 10)
	at(t, small, 2, 5, red)
	at(t, small, 17, 5, blue)
	big := Resize(corners(2), 40, 20)
	size(t, big, 40, 20)
	at(t, big, 2, 10, red)
	at(t, big, 37, 10, blue)
}

func TestResizeAverages(t *testing.T) {
	got := color.NRGBAModel.Convert(Resize(corners(2), 1, 1).At(0, 0)).(color.NRGBA)
	if got.R < 120 || got.R > 135 || got.B < 120 || got.B > 135 || got.A != 255 {
		t.Fatalf("average = %v", got)
	}
}

func TestClearPixelsDontBleed(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	img.SetNRGBA(0, 0, red)
	img.SetNRGBA(1, 0, color.NRGBA{0, 255, 0, 0}) // hidden green
	got := color.NRGBAModel.Convert(Resize(img, 1, 1).At(0, 0)).(color.NRGBA)
	if got.G != 0 || got.R != 255 {
		t.Fatalf("got %v", got)
	}
}

func TestFit(t *testing.T) {
	size(t, Fit(corners(50), 40), 40, 20)
	tall := Rotate(corners(50), 90)
	size(t, Fit(tall, 40), 20, 40)
	if img := corners(5); Fit(img, 100) != image.Image(img) {
		t.Fatal("a small picture should come back as it is")
	}
}

func TestCrop(t *testing.T) {
	c := Crop(corners(10), 10, 0, 10, 10)
	size(t, c, 10, 10)
	at(t, c, 5, 5, blue)
	size(t, Crop(corners(10), 15, 5, 100, 100), 5, 5)
	size(t, Crop(corners(10), 500, 500, 5, 5), 0, 0)
}

func TestRotate(t *testing.T) {
	img := corners(10) // red left, blue right
	r := Rotate(img, 90)
	size(t, r, 10, 20)
	at(t, r, 5, 5, red) // left goes to the top
	at(t, r, 5, 15, blue)
	r = Rotate(img, 180)
	at(t, r, 5, 5, blue)
	r = Rotate(img, 270)
	size(t, r, 10, 20)
	at(t, r, 5, 5, blue)
	r = Rotate(img, -90)
	at(t, r, 5, 5, blue)
	if Rotate(img, 45) != image.Image(img) {
		t.Fatal("45 should change nothing")
	}
}

func TestFlip(t *testing.T) {
	at(t, Flip(corners(10), false), 5, 5, blue)
	img := image.NewNRGBA(image.Rect(0, 0, 1, 2))
	img.SetNRGBA(0, 0, green)
	img.SetNRGBA(0, 1, white)
	at(t, Flip(img, true), 0, 0, white)
}

func TestRoundTrip(t *testing.T) {
	b, err := PNG(corners(10))
	if err != nil {
		t.Fatal(err)
	}
	img, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	at(t, img, 5, 5, red)
	j, err := JPEG(img, 0)
	if err != nil {
		t.Fatal(err)
	}
	if img, err = Decode(j); err != nil || img.Bounds().Dx() != 20 {
		t.Fatalf("jpeg: %v", err)
	}
	if _, err := Decode([]byte("nope")); err == nil {
		t.Fatal("junk should fail")
	}
}

func TestSubImage(t *testing.T) {
	sub := corners(10).SubImage(image.Rect(10, 0, 20, 10))
	at(t, Resize(sub, 5, 5), 2, 2, blue)
}
