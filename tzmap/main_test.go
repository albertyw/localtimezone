package main

import (
	"bytes"
	"errors"
	"image/png"
	"os"
	"regexp"
	"strings"
	"testing"

	localtimezone "github.com/albertyw/localtimezone/v4"
)

// stripes returns a zone per 90 degrees of longitude, in reverse name order
// so that zone indices must be sorted rather than taken in discovery order.
type stripes struct{}

func (stripes) GetZone(p localtimezone.Point) ([]string, error) {
	zone, err := stripes{}.GetOneZone(p)
	return []string{zone}, err
}

func (stripes) GetOneZone(p localtimezone.Point) (string, error) {
	return []string{"Etc/GMT+6", "Etc/GMT+3", "America/New_York", "Africa/Cairo"}[int(p.Lon+180)/90], nil
}

type failing struct{ stripes }

func (failing) GetOneZone(localtimezone.Point) (string, error) {
	return "", localtimezone.ErrOutOfRange
}

func TestPixelCenter(t *testing.T) {
	cases := []struct {
		x, y int
		want localtimezone.Point
	}{
		{0, 0, localtimezone.Point{Lon: -135, Lat: 45}},
		{3, 1, localtimezone.Point{Lon: 135, Lat: -45}},
	}
	for _, c := range cases {
		if got := pixelCenter(c.x, c.y, 4, 2); got != c.want {
			t.Errorf("pixelCenter(%d, %d) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
}

func TestRasterize(t *testing.T) {
	r, err := rasterize(stripes{}, 8, 4)
	if err != nil {
		t.Fatal(err)
	}
	wantZones := []string{"Africa/Cairo", "America/New_York", "Etc/GMT+3", "Etc/GMT+6"}
	if strings.Join(r.zones, ",") != strings.Join(wantZones, ",") {
		t.Fatalf("zones = %v, want %v", r.zones, wantZones)
	}
	wantRow := []uint16{3, 3, 2, 2, 1, 1, 0, 0}
	for y := range r.height {
		for x, want := range wantRow {
			if got := r.pixels[y*r.width+x]; got != want {
				t.Errorf("pixel (%d, %d) = %d, want %d", x, y, got, want)
			}
		}
	}
}

func TestRasterizeMock(t *testing.T) {
	r, err := rasterize(localtimezone.NewMockLocalTimeZone(), 36, 18)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.zones) != 1 || r.zones[0] != localtimezone.MockTimeZone {
		t.Errorf("zones = %v, want [%s]", r.zones, localtimezone.MockTimeZone)
	}
}

func TestRasterizeError(t *testing.T) {
	if _, err := rasterize(failing{}, 4, 2); !errors.Is(err, localtimezone.ErrOutOfRange) {
		t.Errorf("err = %v, want %v", err, localtimezone.ErrOutOfRange)
	}
}

func TestEncodePNG(t *testing.T) {
	r := &raster{width: 2, height: 2, zones: make([]string, 300), pixels: []uint16{0, 1, 256, 299}}
	data, err := encodePNG(r)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range r.pixels {
		red, green, _, alpha := img.At(i%2, i/2).RGBA()
		if got := uint16(red>>8)<<8 | uint16(green>>8); got != want || alpha != 0xffff {
			t.Errorf("pixel %d = %d (alpha %d), want %d", i, got, alpha, want)
		}
	}
}

func TestRender(t *testing.T) {
	r, err := rasterize(stripes{}, 8, 4)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := render(&buf, r); err != nil {
		t.Fatal(err)
	}
	page := buf.String()
	for _, want := range []string{
		"<title>localtimezone World Map</title>",
		`const VERSION = "` + localtimezone.TZBoundaryVersion + `";`,
		`const ZONES = ["Africa/Cairo","America/New_York","Etc/GMT+3","Etc/GMT+6"];`,
		`const RASTER = "data:image/png;base64,iVBOR`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("rendered page is missing %q", want)
		}
	}
}

func TestMapVersion(t *testing.T) {
	page, err := os.ReadFile("map.html")
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`const VERSION = "([^"]*)";`).FindSubmatch(page)
	if match == nil {
		t.Fatal("map.html has no VERSION variable")
	}
	if got := string(match[1]); got != localtimezone.TZBoundaryVersion {
		t.Errorf("map.html version = %q, want %q; regenerate it with make map", got, localtimezone.TZBoundaryVersion)
	}
}
