// Command tzmap renders the embedded timezone data as an interactive HTML
// world map. Every pixel of an equirectangular raster is looked up with
// GetOneZone, so the map shows exactly what the library returns.
package main

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"os"
	"runtime"
	"slices"
	"sync"
	"text/template"

	localtimezone "github.com/albertyw/localtimezone/v4"
)

//go:embed map.html.tmpl
var pageTemplate string

// raster is an equirectangular grid of zone indices. Row 0 is the northern
// edge and column 0 is the antimeridian at -180 degrees longitude.
type raster struct {
	width, height int
	zones         []string
	pixels        []uint16
}

func main() {
	out := flag.String("out", "tzmap/map.html", "path of the HTML file to write")
	width := flag.Int("width", 3600, "raster width in pixels; the height is half of it")
	flag.Parse()

	r, err := rasterize(localtimezone.NewLocalTimeZone(), *width, *width/2)
	if err != nil {
		log.Fatal(err)
	}
	f, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := render(f, r); err != nil {
		log.Fatal(err)
	}
}

// pixelCenter returns the coordinates at the center of a raster pixel.
func pixelCenter(x, y, width, height int) localtimezone.Point {
	return localtimezone.Point{
		Lon: -180 + (float64(x)+0.5)*360/float64(width),
		Lat: 90 - (float64(y)+0.5)*180/float64(height),
	}
}

// rasterize looks up the zone of every pixel in parallel. Zone indices follow
// the sorted zone names so the output is deterministic.
func rasterize(tz localtimezone.LocalTimeZone, width, height int) (*raster, error) {
	names := make([]string, width*height)
	rows := make(chan int)
	var (
		wg       sync.WaitGroup
		errOnce  sync.Once
		firstErr error
	)
	for range runtime.NumCPU() {
		wg.Go(func() {
			for y := range rows {
				for x := range width {
					zone, err := tz.GetOneZone(pixelCenter(x, y, width, height))
					if err != nil {
						errOnce.Do(func() { firstErr = err })
					}
					names[y*width+x] = zone
				}
			}
		})
	}
	for y := range height {
		rows <- y
	}
	close(rows)
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}

	zones := slices.Clone(names)
	slices.Sort(zones)
	zones = slices.Compact(zones)
	if len(zones) > 1<<16 {
		return nil, fmt.Errorf("too many zones to index: %d", len(zones))
	}
	index := make(map[string]uint16, len(zones))
	for i, zone := range zones {
		index[zone] = uint16(i)
	}
	pixels := make([]uint16, len(names))
	for i, name := range names {
		pixels[i] = index[name]
	}
	return &raster{width: width, height: height, zones: zones, pixels: pixels}, nil
}

// encodePNG stores each pixel's zone index in its red (high byte) and green
// (low byte) channels, which the page decodes back into zone indices.
func encodePNG(r *raster) ([]byte, error) {
	img := image.NewNRGBA(image.Rect(0, 0, r.width, r.height))
	for i, idx := range r.pixels {
		img.SetNRGBA(i%r.width, i/r.width, color.NRGBA{R: uint8(idx >> 8), G: uint8(idx), A: 255})
	}
	var buf bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	if err := encoder.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func render(w io.Writer, r *raster) error {
	pngData, err := encodePNG(r)
	if err != nil {
		return err
	}
	zones, err := json.Marshal(r.zones)
	if err != nil {
		return err
	}
	tmpl, err := template.New("map").Parse(pageTemplate)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, map[string]any{
		"Zones": string(zones),
		"PNG":   base64.StdEncoding.EncodeToString(pngData),
	})
}
