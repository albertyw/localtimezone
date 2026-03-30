// Generates data_mock.h3.s2 — a small H3 dataset mapping all base cells
// to "America/Los_Angeles" for testing purposes.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/klauspost/compress/s2"
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
	var tmp [4]byte
	binary.LittleEndian.PutUint16(tmp[:2], 1) // 1 timezone string
	buf.Write(tmp[:2])

	// String table
	binary.LittleEndian.PutUint16(tmp[:2], uint16(len(tzNameBytes)))
	buf.Write(tmp[:2])
	buf.Write(tzNameBytes)

	// Cell data: bulk write using direct byte encoding
	binary.LittleEndian.PutUint32(tmp[:4], uint32(len(cells)))
	buf.Write(tmp[:4])

	entryBuf := make([]byte, len(cells)*10)
	for i, c := range cells {
		base := i * 10
		binary.LittleEndian.PutUint64(entryBuf[base:base+8], uint64(c))
		binary.LittleEndian.PutUint16(entryBuf[base+8:base+10], 0) // index 0 = "America/Los_Angeles"
	}
	buf.Write(entryBuf)

	// S2 compress
	var compressed bytes.Buffer
	w := s2.NewWriter(&compressed, s2.WriterBestCompression())
	if _, err := w.Write(buf.Bytes()); err != nil {
		log.Fatalf("s2 write: %v", err)
	}
	if err := w.Close(); err != nil {
		log.Fatalf("s2 close: %v", err)
	}

	err = os.WriteFile("data_mock.h3.s2", compressed.Bytes(), 0644)
	if err != nil {
		log.Fatalf("write file: %v", err)
	}
	fmt.Printf("Wrote data_mock.h3.s2 (%d bytes)\n", compressed.Len())
}
