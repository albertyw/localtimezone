// Generates data_mock.h3.gz — a small H3 dataset mapping cells around
// Los Angeles to "America/Los_Angeles" for testing purposes.
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/uber/h3-go/v4"
)

const h3Resolution = 7

func main() {
	// Generate cells for a small polygon around Los Angeles
	geoPolygon := h3.GeoPolygon{
		GeoLoop: h3.GeoLoop{
			h3.NewLatLng(33.5, -118.8),
			h3.NewLatLng(33.5, -117.5),
			h3.NewLatLng(34.5, -117.5),
			h3.NewLatLng(34.5, -118.8),
			h3.NewLatLng(33.5, -118.8),
		},
	}

	cells, err := h3.PolygonToCells(geoPolygon, h3Resolution)
	if err != nil {
		log.Fatalf("PolygonToCells: %v", err)
	}
	fmt.Printf("Generated %d cells for mock LA area\n", len(cells))

	// Build binary format
	tzName := "America/Los_Angeles"
	tzNameBytes := []byte(tzName)

	// Sort cells
	sort.Slice(cells, func(i, j int) bool {
		return cells[i] < cells[j]
	})

	var buf bytes.Buffer

	// Header
	buf.Write([]byte("H3TZ"))
	buf.WriteByte(1) // Version
	buf.WriteByte(byte(h3Resolution))
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // 1 timezone string

	// String table
	binary.Write(&buf, binary.LittleEndian, uint16(len(tzNameBytes)))
	buf.Write(tzNameBytes)

	// Cell data
	binary.Write(&buf, binary.LittleEndian, uint32(len(cells)))
	for _, c := range cells {
		binary.Write(&buf, binary.LittleEndian, int64(c))
		binary.Write(&buf, binary.LittleEndian, uint16(0)) // index 0 = "America/Los_Angeles"
	}

	// Gzip compress
	var compressed bytes.Buffer
	gzipper, err := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
	if err != nil {
		log.Fatalf("gzip writer: %v", err)
	}
	if _, err := gzipper.Write(buf.Bytes()); err != nil {
		log.Fatalf("gzip write: %v", err)
	}
	if err := gzipper.Close(); err != nil {
		log.Fatalf("gzip close: %v", err)
	}

	err = os.WriteFile("data_mock.h3.gz", compressed.Bytes(), 0644)
	if err != nil {
		log.Fatalf("write file: %v", err)
	}
	fmt.Printf("Wrote data_mock.h3.gz (%d bytes)\n", compressed.Len())
}
