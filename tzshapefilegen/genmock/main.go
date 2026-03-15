// Generates data_mock.h3.gz — a small H3 dataset mapping all base cells
// to "America/Los_Angeles" for testing purposes.
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
	// Use all 122 resolution-0 base cells so every point on Earth
	// resolves to "America/Los_Angeles" via parent hierarchy lookup.
	cells, err := h3.Res0Cells()
	if err != nil {
		log.Fatalf("Res0Cells: %v", err)
	}
	fmt.Printf("Using %d resolution-0 base cells for mock data\n", len(cells))

	// Sort cells
	sort.Slice(cells, func(i, j int) bool {
		return cells[i] < cells[j]
	})

	// Build binary format
	tzName := "America/Los_Angeles"
	tzNameBytes := []byte(tzName)

	var buf bytes.Buffer

	// Header
	buf.Write([]byte("H3TZ"))
	buf.WriteByte(1) // Version
	buf.WriteByte(byte(h3Resolution))
	if err := binary.Write(&buf, binary.LittleEndian, uint16(1)); err != nil {
		log.Fatalf("write string count: %v", err)
	}

	// String table
	if err := binary.Write(&buf, binary.LittleEndian, uint16(len(tzNameBytes))); err != nil {
		log.Fatalf("write string length: %v", err)
	}
	buf.Write(tzNameBytes)

	// Cell data
	if err := binary.Write(&buf, binary.LittleEndian, uint32(len(cells))); err != nil {
		log.Fatalf("write cell count: %v", err)
	}
	for _, c := range cells {
		if err := binary.Write(&buf, binary.LittleEndian, int64(c)); err != nil {
			log.Fatalf("write cell: %v", err)
		}
		if err := binary.Write(&buf, binary.LittleEndian, uint16(0)); err != nil {
			log.Fatalf("write tz index: %v", err)
		}
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
