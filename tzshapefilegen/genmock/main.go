// Generates data_mock.h3.s2 — a small H3 dataset mapping all base cells
// to "America/Los_Angeles" for testing purposes.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"slices"

	"github.com/klauspost/compress/s2"
	"github.com/uber/h3-go/v4"
)

const h3Resolution = 7

// entrySize is the size of one encoded cell entry: 8 bytes for the int64 cell
// followed by 2 bytes for the uint16 timezone index.
const entrySize = 10

// buildMockData builds the compressed mock H3 dataset.
func buildMockData() ([]byte, error) {
	// Use all 122 resolution-0 base cells so every point on Earth
	// resolves to "America/Los_Angeles" via parent hierarchy lookup.
	cells, err := h3.Res0Cells()
	if err != nil {
		return nil, fmt.Errorf("Res0Cells: %w", err)
	}
	fmt.Printf("Using %d resolution-0 base cells for mock data\n", len(cells))

	// Sort cells for binary search
	slices.Sort(cells)

	// Build binary format
	tzNameBytes := []byte("America/Los_Angeles")
	var u16 [2]byte
	var u32 [4]byte
	var buf bytes.Buffer

	// Header
	buf.Write([]byte("H3TZ"))
	buf.WriteByte(1) // Version
	buf.WriteByte(byte(h3Resolution))
	binary.LittleEndian.PutUint16(u16[:], 1) // 1 timezone string
	buf.Write(u16[:])

	// String table
	binary.LittleEndian.PutUint16(u16[:], uint16(len(tzNameBytes)))
	buf.Write(u16[:])
	buf.Write(tzNameBytes)

	// Cell data: bulk write using direct byte encoding
	binary.LittleEndian.PutUint32(u32[:], uint32(len(cells)))
	buf.Write(u32[:])

	entryBuf := make([]byte, len(cells)*entrySize)
	for i, c := range cells {
		base := i * entrySize
		binary.LittleEndian.PutUint64(entryBuf[base:base+8], uint64(c))
		binary.LittleEndian.PutUint16(entryBuf[base+8:base+10], 0) // index 0 = "America/Los_Angeles"
	}
	buf.Write(entryBuf)

	// S2 compress (block format)
	return s2.EncodeBest(nil, buf.Bytes()), nil
}

func run() error {
	compressed, err := buildMockData()
	if err != nil {
		return err
	}
	if err := os.WriteFile("data_mock.h3.s2", compressed, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Wrote data_mock.h3.s2 (%d bytes)\n", len(compressed))
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
