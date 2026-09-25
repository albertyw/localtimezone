package main

import (
	"hash/fnv"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // pin offsets to Go's tzdata rather than the host's
)

// These colors match the light theme of map.html.tmpl.
var (
	borderColor = color.NRGBA{R: 70, G: 84, B: 92, A: 255}
	coastColor  = color.NRGBA{R: 38, G: 54, B: 62, A: 255}
	seaEven     = color.NRGBA{R: 212, G: 226, B: 233, A: 255}
	seaOdd      = color.NRGBA{R: 197, G: 215, B: 224, A: 255}
)

var etcZone = regexp.MustCompile(`^Etc/GMT([+-]\d+)?$`)

// isComputed reports whether a zone is a nautical fallback rather than an
// official timezone.
func isComputed(name string) bool {
	return strings.HasPrefix(name, "Etc/")
}

// standardOffset returns a zone's standard UTC offset in minutes, taken as
// the smaller of its offsets in January and July of the given year.
// POSIX-style Etc names invert the sign: Etc/GMT+5 is UTC-05:00.
func standardOffset(name string, year int) (int, error) {
	if m := etcZone.FindStringSubmatch(name); m != nil {
		if m[1] == "" {
			return 0, nil
		}
		hours, err := strconv.Atoi(m[1])
		return -hours * 60, err
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return 0, err
	}
	_, jan := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC).In(loc).Zone()
	_, jul := time.Date(year, time.July, 1, 0, 0, 0, 0, time.UTC).In(loc).Zone()
	return min(jan, jul) / 60, nil
}

// hslToRGB converts a hue in degrees and saturation and lightness in [0, 1].
func hslToRGB(h, s, l float64) color.NRGBA {
	a := s * min(l, 1-l)
	f := func(n float64) uint8 {
		k := math.Mod(n+h/30, 12)
		return uint8(math.Round((l - a*max(-1, min(k-3, 9-k, 1))) * 255))
	}
	return color.NRGBA{R: f(0), G: f(8), B: f(4), A: 255}
}

// zoneColor mirrors zoneColor in map.html.tmpl. Neighboring offsets land far
// apart on the color wheel, and each zone gets a small lightness nudge so
// same-offset neighbors stay distinguishable.
func zoneColor(name string, year int) color.NRGBA {
	offset, err := standardOffset(name, year)
	if isComputed(name) {
		if (offset/60)%2 != 0 {
			return seaOdd
		}
		return seaEven
	}
	hue, sat := 0.0, 0.0
	if err == nil {
		hue = math.Mod(math.Mod(200+float64(offset)/60*137.5, 360)+360, 360)
		sat = 0.5
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	jitter := (float64(h.Sum32())/4294967296 - 0.5) * 0.1
	return hslToRGB(hue, sat, 0.76+jitter)
}

// renderImage draws the map as a PNG the way map.html.tmpl paints it: zones
// filled by color, with borders between zones and a coastline between
// official zones and nautical ones.
func renderImage(w io.Writer, r *raster, year int) error {
	colors := make([]color.NRGBA, len(r.zones))
	computed := make([]bool, len(r.zones))
	for i, zone := range r.zones {
		colors[i] = zoneColor(zone, year)
		computed[i] = isComputed(zone)
	}
	img := image.NewNRGBA(image.Rect(0, 0, r.width, r.height))
	for y := range r.height {
		for x := range r.width {
			i := y*r.width + x
			id := r.pixels[i]
			c := colors[id]
			var neighbors []uint16
			if x+1 < r.width {
				neighbors = append(neighbors, r.pixels[i+1])
			}
			if y+1 < r.height {
				neighbors = append(neighbors, r.pixels[i+r.width])
			}
			for _, other := range neighbors {
				if other == id {
					continue
				}
				if computed[id] != computed[other] {
					c = coastColor
					break
				}
				if !computed[id] {
					c = borderColor
				}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	return encoder.Encode(w, img)
}
