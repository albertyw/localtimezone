package main

import (
	"encoding/binary"
	"os"
	"strings"
	"testing"

	"github.com/klauspost/compress/s2"
	"github.com/uber/h3-go/v4"
)

// parseMockData reads back the binary format produced by buildMockData.
func parseMockData(t *testing.T, compressed []byte) (names []string, cells []h3.Cell) {
	t.Helper()
	data, err := s2.Decode(nil, compressed)
	if err != nil {
		t.Fatalf("cannot decompress: %v", err)
	}
	if string(data[0:4]) != "H3TZ" {
		t.Fatalf("unexpected magic %q", string(data[0:4]))
	}
	if data[4] != 1 {
		t.Errorf("unexpected version %d", data[4])
	}
	if data[5] != h3Resolution {
		t.Errorf("unexpected resolution %d", data[5])
	}
	nameCount := int(binary.LittleEndian.Uint16(data[6:8]))
	offset := 8
	for range nameCount {
		length := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
		offset += 2
		names = append(names, string(data[offset:offset+length]))
		offset += length
	}
	cellCount := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4
	if len(data) != offset+cellCount*10 {
		t.Fatalf("unexpected data length %d, want %d", len(data), offset+cellCount*10)
	}
	for i := range cellCount {
		base := offset + i*10
		cells = append(cells, h3.Cell(binary.LittleEndian.Uint64(data[base:base+8])))
		if idx := binary.LittleEndian.Uint16(data[base+8 : base+10]); idx != 0 {
			t.Errorf("expected timezone index 0, got %d", idx)
		}
	}
	return names, cells
}

func TestBuildMockData(t *testing.T) {
	compressed, err := buildMockData()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	names, cells := parseMockData(t, compressed)
	if len(names) != 1 || names[0] != "America/Los_Angeles" {
		t.Errorf("unexpected string table %v", names)
	}

	res0Cells, err := h3.Res0Cells()
	if err != nil {
		t.Fatalf("cannot get res0 cells: %v", err)
	}
	if len(cells) != len(res0Cells) {
		t.Errorf("expected %d cells, got %d", len(res0Cells), len(cells))
	}
	for i := 1; i < len(cells); i++ {
		if cells[i-1] >= cells[i] {
			t.Fatalf("cells are not sorted at index %d", i)
		}
	}
}

func TestRun(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	compressed, err := os.ReadFile("data_mock.h3.s2")
	if err != nil {
		t.Fatalf("cannot read data_mock.h3.s2: %v", err)
	}
	names, cells := parseMockData(t, compressed)
	if len(names) != 1 || names[0] != "America/Los_Angeles" {
		t.Errorf("unexpected string table %v", names)
	}
	if len(cells) == 0 {
		t.Errorf("expected cells in mock data")
	}
}

func TestRunWriteError(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("data_mock.h3.s2", 0755); err != nil {
		t.Fatalf("cannot create blocking directory: %v", err)
	}

	err := run()
	if err == nil || !strings.Contains(err.Error(), "write file") {
		t.Errorf("expected write error, got %v", err)
	}
}
