package main

import (
	"bytes"
	"image/color"
	"image/png"
	"testing"
)

func TestStandardOffset(t *testing.T) {
	cases := []struct {
		name string
		want int
	}{
		{"Etc/GMT", 0},
		{"Etc/GMT+5", -300},
		{"Etc/GMT-12", 720},
		{"America/New_York", -300},
		{"Asia/Kolkata", 330},
		{"Australia/Sydney", 600},
	}
	for _, c := range cases {
		got, err := standardOffset(c.name, 2026)
		if err != nil || got != c.want {
			t.Errorf("standardOffset(%q) = %d, %v, want %d", c.name, got, err, c.want)
		}
	}
	if _, err := standardOffset("Nowhere/Atlantis", 2026); err == nil {
		t.Error("standardOffset of an unknown zone returned no error")
	}
}

func TestHSLToRGB(t *testing.T) {
	cases := []struct {
		h, s, l float64
		want    color.NRGBA
	}{
		{0, 0, 0.5, color.NRGBA{R: 128, G: 128, B: 128, A: 255}},
		{0, 1, 0.5, color.NRGBA{R: 255, G: 0, B: 0, A: 255}},
		{120, 1, 0.5, color.NRGBA{R: 0, G: 255, B: 0, A: 255}},
		{240, 1, 0.25, color.NRGBA{R: 0, G: 0, B: 128, A: 255}},
	}
	for _, c := range cases {
		if got := hslToRGB(c.h, c.s, c.l); got != c.want {
			t.Errorf("hslToRGB(%v, %v, %v) = %v, want %v", c.h, c.s, c.l, got, c.want)
		}
	}
}

func TestZoneColor(t *testing.T) {
	cases := []struct {
		name string
		want color.NRGBA
	}{
		// Official zones match the colors map.html.tmpl paints in a browser.
		{"America/New_York", color.NRGBA{R: 145, G: 154, B: 218, A: 255}},
		{"Asia/Kolkata", color.NRGBA{R: 180, G: 183, B: 230, A: 255}},
		{"Etc/GMT", seaEven},
		{"Etc/GMT+5", seaOdd},
		{"Etc/GMT-2", seaEven},
	}
	for _, c := range cases {
		if got := zoneColor(c.name, 2026); got != c.want {
			t.Errorf("zoneColor(%q) = %v, want %v", c.name, got, c.want)
		}
	}
	if got := zoneColor("Nowhere/Atlantis", 2026); got.R != got.G || got.G != got.B {
		t.Errorf("zoneColor of an unknown zone = %v, want a gray", got)
	}
}

func TestRenderImage(t *testing.T) {
	r, err := rasterize(stripes{}, 8, 4)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := renderImage(&buf, r, 2026); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if got := img.Bounds().Size(); got.X != 8 || got.Y != 4 {
		t.Fatalf("image size = %v, want 8x4", got)
	}
	// Columns run Etc/GMT+6, Etc/GMT+3, America/New_York, Africa/Cairo in
	// pairs; each pair's right pixel borders the next zone.
	wantRow := []color.NRGBA{
		seaEven, seaEven,
		seaOdd, coastColor,
		zoneColor("America/New_York", 2026), borderColor,
		zoneColor("Africa/Cairo", 2026), zoneColor("Africa/Cairo", 2026),
	}
	for y := range 4 {
		for x, want := range wantRow {
			if got := color.NRGBAModel.Convert(img.At(x, y)); got != want {
				t.Errorf("pixel (%d, %d) = %v, want %v", x, y, got, want)
			}
		}
	}
}
