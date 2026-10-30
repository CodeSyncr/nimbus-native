package toolchain

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg" // icons may be JPEG
	"image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// App icons and launch screens: one square picture becomes the iOS App
// Store icon and Android's adaptive launcher icons; one color becomes the
// launch screen's background on both.

// androidDensities are the launcher icon sizes, by density bucket, of a
// 108 dp adaptive icon layer.
var androidDensities = []struct {
	dir  string
	size int
}{{"mipmap-mdpi", 108}, {"mipmap-hdpi", 162}, {"mipmap-xhdpi", 216}, {"mipmap-xxhdpi", 324}, {"mipmap-xxxhdpi", 432}}

// ReadIcon loads a square PNG or JPEG, ideally 1024×1024 or larger.
func ReadIcon(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w (use a PNG or JPEG)", path, err)
	}
	b := img.Bounds()
	if b.Dx() != b.Dy() {
		return nil, fmt.Errorf("%s is %d×%d: an app icon must be square", path, b.Dx(), b.Dy())
	}
	return img, nil
}

// ParseColor reads "#rrggbb" (or "rrggbb").
func ParseColor(s string) (color.RGBA, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if !regexp.MustCompile(`^[0-9a-fA-F]{6}$`).MatchString(s) {
		return color.RGBA{}, fmt.Errorf("%q is not a color like #6d28d9", s)
	}
	v, _ := strconv.ParseUint(s, 16, 32)
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}, nil
}

// WriteIcons writes the project's app icons from icon over bg, and sets
// its launch screens' background to launch.
func WriteIcons(root string, icon image.Image, bg, launch color.RGBA) error {
	// iOS: one 1024 pt icon, flattened (the App Store rejects transparency).
	ios := filepath.Join(root, "native/ios/Runner/Assets.xcassets")
	flat := image.NewRGBA(image.Rect(0, 0, 1024, 1024))
	draw.Draw(flat, flat.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	draw.Draw(flat, flat.Bounds(), scale(icon, 1024), image.Point{}, draw.Over)
	if err := writePNG(filepath.Join(ios, "AppIcon.appiconset/icon-1024.png"), flat); err != nil {
		return err
	}
	if err := writeText(filepath.Join(ios, "AppIcon.appiconset/Contents.json"), iosIconContents); err != nil {
		return err
	}
	if err := writeText(filepath.Join(ios, "LaunchBackground.colorset/Contents.json"), colorSet(launch)); err != nil {
		return err
	}

	// Android: an adaptive icon, the picture inside the 72 dp safe zone of a
	// 108 dp layer, over the background color.
	res := filepath.Join(root, "native/android/app/src/main/res")
	for _, d := range androidDensities {
		layer := image.NewRGBA(image.Rect(0, 0, d.size, d.size))
		inner := d.size * 2 / 3
		off := (d.size - inner) / 2
		draw.Draw(layer, image.Rect(off, off, off+inner, off+inner), scale(icon, inner), image.Point{}, draw.Over)
		if err := writePNG(filepath.Join(res, d.dir, "ic_launcher_foreground.png"), layer); err != nil {
			return err
		}
	}
	files := map[string]string{
		"mipmap-anydpi-v26/ic_launcher.xml":       adaptiveIcon,
		"mipmap-anydpi-v26/ic_launcher_round.xml": adaptiveIcon,
		"values/launch.xml": fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<!-- Written by nimbus native icon: the launch screen and icon backgrounds. -->
<resources>
    <color name="launch_background">#%02x%02x%02x</color>
    <color name="icon_background">#%02x%02x%02x</color>
    <style name="LaunchTheme" parent="@android:style/Theme.DeviceDefault.Light.NoActionBar">
        <item name="android:windowBackground">@color/launch_background</item>
    </style>
</resources>
`, launch.R, launch.G, launch.B, bg.R, bg.G, bg.B),
		"values-v31/launch.xml": `<?xml version="1.0" encoding="utf-8"?>
<!-- Written by nimbus native icon: Android 12's splash screen. -->
<resources>
    <style name="LaunchTheme" parent="@android:style/Theme.DeviceDefault.Light.NoActionBar">
        <item name="android:windowBackground">@color/launch_background</item>
        <item name="android:windowSplashScreenBackground">@color/launch_background</item>
        <item name="android:windowSplashScreenIconBackgroundColor">@color/launch_background</item>
    </style>
</resources>
`,
	}
	for name, content := range files {
		if err := writeText(filepath.Join(res, name), content); err != nil {
			return err
		}
	}
	return nil
}

const adaptiveIcon = `<?xml version="1.0" encoding="utf-8"?>
<adaptive-icon xmlns:android="http://schemas.android.com/apk/res/android">
    <background android:drawable="@color/icon_background" />
    <foreground android:drawable="@mipmap/ic_launcher_foreground" />
</adaptive-icon>
`

const iosIconContents = `{
  "images" : [
    {
      "filename" : "icon-1024.png",
      "idiom" : "universal",
      "platform" : "ios",
      "size" : "1024x1024"
    }
  ],
  "info" : {
    "author" : "xcode",
    "version" : 1
  }
}
`

func colorSet(c color.RGBA) string {
	return fmt.Sprintf(`{
  "colors" : [
    {
      "color" : {
        "color-space" : "srgb",
        "components" : {
          "alpha" : "1.000",
          "blue" : "0x%02X",
          "green" : "0x%02X",
          "red" : "0x%02X"
        }
      },
      "idiom" : "universal"
    }
  ],
  "info" : {
    "author" : "xcode",
    "version" : 1
  }
}
`, c.B, c.G, c.R)
}

// DefaultIcon is a new project's icon: a white plus on fg, on a
// transparent square (the background color fills behind it).
func DefaultIcon(fg color.RGBA) image.Image {
	const n = 1024
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	// A soft rounded tile, lighter at the top.
	for y := 0; y < n; y++ {
		t := float64(y) / n
		c := color.RGBA{lerp(fg.R, 255, 0.18*(1-t)), lerp(fg.G, 255, 0.18*(1-t)), lerp(fg.B, 255, 0.18*(1-t)), 255}
		for x := 0; x < n; x++ {
			if insideRounded(x, y, 112, 112, n-224, n-224, 200) {
				img.Set(x, y, c)
			}
		}
	}
	white := color.RGBA{255, 255, 255, 255}
	bar, long := 104, 440
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			h := insideRounded(x, y, (n-long)/2, (n-bar)/2, long, bar, bar/2)
			v := insideRounded(x, y, (n-bar)/2, (n-long)/2, bar, long, bar/2)
			if h || v {
				img.Set(x, y, white)
			}
		}
	}
	return img
}

func lerp(a, b uint8, t float64) uint8 { return uint8(float64(a) + (float64(b)-float64(a))*t) }

func insideRounded(x, y, rx, ry, w, h, r int) bool {
	if x < rx || y < ry || x >= rx+w || y >= ry+h {
		return false
	}
	cx := math.Max(float64(rx+r), math.Min(float64(x), float64(rx+w-r-1)))
	cy := math.Max(float64(ry+r), math.Min(float64(y), float64(ry+h-r-1)))
	dx, dy := float64(x)-cx, float64(y)-cy
	return dx*dx+dy*dy <= float64(r*r)
}

// scale resizes a square image to size×size by averaging the source pixels
// under each target pixel (good for shrinking, fine for small growth).
func scale(src image.Image, size int) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	sx := float64(b.Dx()) / float64(size)
	sy := float64(b.Dy()) / float64(size)
	for y := 0; y < size; y++ {
		y0, y1 := int(float64(y)*sy), int(math.Ceil(float64(y+1)*sy))
		for x := 0; x < size; x++ {
			x0, x1 := int(float64(x)*sx), int(math.Ceil(float64(x+1)*sx))
			var r, g, bl, a, n float64
			for yy := y0; yy < y1 && yy < b.Dy(); yy++ {
				for xx := x0; xx < x1 && xx < b.Dx(); xx++ {
					cr, cg, cb, ca := src.At(b.Min.X+xx, b.Min.Y+yy).RGBA()
					r, g, bl, a, n = r+float64(cr), g+float64(cg), bl+float64(cb), a+float64(ca), n+1
				}
			}
			if n == 0 {
				continue
			}
			dst.Set(x, y, color.RGBA64{uint16(r / n), uint16(g / n), uint16(bl / n), uint16(a / n)})
		}
	}
	return dst
}

func writePNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func writeText(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// LaunchColor is the project's current launch screen color (white if unset).
func LaunchColor(root string) color.RGBA {
	b, err := os.ReadFile(filepath.Join(root, "native/android/app/src/main/res/values/launch.xml"))
	if err == nil {
		if m := regexp.MustCompile(`name="launch_background">#([0-9a-fA-F]{6})<`).FindSubmatch(b); m != nil {
			if c, err := ParseColor(string(m[1])); err == nil {
				return c
			}
		}
	}
	return color.RGBA{255, 255, 255, 255}
}

// AddFont copies a .ttf or .otf into the project's iOS app (Runner/Fonts,
// registered at launch) and Android app (assets/fonts, also served to the
// web), named family. It returns the family name to use in
// Style.FontFamily.
func AddFont(root, file, family string) (string, error) {
	ext := strings.ToLower(filepath.Ext(file))
	if ext != ".ttf" && ext != ".otf" {
		return "", fmt.Errorf("%s: fonts are .ttf or .otf files", file)
	}
	if family == "" {
		family = strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	}
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 _-]*$`).MatchString(family) {
		return "", fmt.Errorf("%q: name fonts with letters, digits, spaces, - and _", family)
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	for _, dir := range []string{"native/ios/Runner/Fonts", "native/android/app/src/main/assets/fonts"} {
		p := filepath.Join(root, dir, family+ext)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(p, b, 0o644); err != nil {
			return "", err
		}
	}
	return family, nil
}
