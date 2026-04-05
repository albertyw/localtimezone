package localtimezone

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"
	"testing"

	"github.com/klauspost/compress/s2"
	"github.com/uber/h3-go/v4"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestNewLocalTimeZonePanic(t *testing.T) {
	tempTZData := TZData
	TZData = []byte("asdf")
	defer func() {
		TZData = tempTZData
		if r := recover(); r == nil {
			t.Errorf("expected a panic; got no panic")
		}
	}()
	NewLocalTimeZone()
}

func TestLoadError(t *testing.T) {
	client := NewLocalTimeZone()
	c, ok := client.(*localTimeZone)
	if !ok {
		t.Errorf("error when initializing client")
	}

	badData := []byte("asdf")
	if err := c.load(badData); err == nil {
		t.Errorf("expected error when loading malformed data")
	}

	var badData2 bytes.Buffer
	writer := s2.NewWriter(&badData2)
	if _, err := writer.Write([]byte("asdf")); err != nil {
		t.Errorf("cannot write to s2, got error %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Errorf("cannot close s2 writer, got error %v", err)
	}
	if err := c.load(badData2.Bytes()); err == nil {
		t.Errorf("expected error when loading malformed data")
	}
}

func TestParallelNewLocalTimeZone(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			NewLocalTimeZone()
		}()
	}
	wg.Wait()
}

type result struct {
	zones []string
	err   error
}

var _tt = []struct {
	name  string
	point Point
	result
}{
	{
		"Riga",
		Point{24.105078, 56.946285},
		result{
			zones: []string{"Europe/Riga"},
			err:   nil,
		},
	},
	{
		"Tokyo",
		Point{139.7594549, 35.6828387},
		result{
			zones: []string{"Asia/Tokyo"},
			err:   nil,
		},
	},
	{
		"Urumqi/Shanghai",
		Point{87.319461, 43.419754},
		result{
			zones: []string{"Asia/Shanghai", "Asia/Urumqi"},
			err:   nil,
		},
	},
	{
		"Baker Island",
		Point{-176.474331436, 0.190165906},
		result{
			zones: []string{"Etc/GMT+12"},
			err:   nil,
		},
	},
	{
		"Asuncion",
		Point{-57.637517, -25.335772},
		result{
			zones: []string{"America/Asuncion"},
			err:   nil,
		},
	},
	{
		"Across the river from Asuncion",
		Point{-57.681572, -25.351069},
		result{
			zones: []string{"America/Argentina/Cordoba"},
			err:   nil,
		},
	},
	{
		"Singapore",
		Point{103.811988, 1.466482},
		result{
			zones: []string{"Asia/Singapore"},
			err:   nil,
		},
	},
	{
		"Across the river from Singapore",
		Point{103.768481, 1.462410},
		result{
			zones: []string{"Asia/Kuala_Lumpur"},
			err:   nil,
		},
	},
	{
		"Broken Timezone",
		Point{360.0, 360.0},
		result{
			zones: []string{""},
			err:   ErrOutOfRange,
		},
	},
	{
		"Null Island",
		Point{0.0, 0.0},
		result{
			zones: []string{"Etc/GMT"},
			err:   nil,
		},
	},
}

func TestGetZone(t *testing.T) {
	t.Parallel()
	z := NewLocalTimeZone()
	for _, tc := range _tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tzids, err := z.GetZone(tc.point)
			if tc.err != nil {
				if err != tc.err {
					t.Errorf("expected err %v; got %v", tc.err, err)
				}
				return
			}
			if len(tzids) != len(tc.zones) {
				t.Errorf("expected %d zones; got %d", len(tc.zones), len(tzids))
			}
			for i, zone := range tc.zones {
				if tzids[i] != zone {
					t.Errorf("expected zone %s; got %s", zone, tzids[i])
				}
			}
		})
	}
}

func TestGetOneZone(t *testing.T) {
	t.Parallel()
	z := NewLocalTimeZone()
	for _, tc := range _tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tzid, err := z.GetOneZone(tc.point)
			if err != tc.err {
				t.Errorf("expected err %v; got %v", tc.err, err)
			}
			found := false
			for _, zone := range tc.zones {
				if tzid == zone {
					found = true
				}
			}
			if !found {
				t.Errorf("expected one of zones %s; got %s", tc.zones, tzid)
			}
		})
	}
}

func TestMockLocalTimeZone(t *testing.T) {
	z := NewMockLocalTimeZone()
	for _, tc := range _tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tzids, err := z.GetZone(tc.point)
			if tc.err != nil {
				if err != tc.err {
					t.Errorf("expected err %v; got %v", tc.err, err)
				}
				return
			}
			if len(tzids) != 1 {
				t.Errorf("expected 1 zone; got %d", len(tzids))
			}
			if tzids[0] != MockTimeZone {
				t.Errorf("expected zone America/Los_Angeles; got %s", tzids[0])
			}
		})
	}
}

func TestMockLocalTimeZonePanic(t *testing.T) {
	tempMockTZData := MockTZData
	MockTZData = []byte("asdf")
	defer func() {
		MockTZData = tempMockTZData
		if r := recover(); r == nil {
			t.Errorf("expected a panic; got no panic")
		}
	}()
	NewMockLocalTimeZone()
}

func BenchmarkZones(b *testing.B) {
	z := NewLocalTimeZone()

	// Ensure client has finished loading data
	if _, err := z.GetZone(Point{0, 0}); err != nil {
		b.Errorf("cannot initialize timezone client: %v", err)
	}

	b.Run("test cases", func(b *testing.B) {
		points := make([]Point, 0, len(_tt))
		for _, tc := range _tt {
			if tc.err != nil {
				continue
			}
			points = append(points, tc.point)
		}
		n := 0
		for b.Loop() {
			point := points[n%len(points)]
			_, err := z.GetZone(point)
			if err != nil {
				b.Errorf("point %v did not return a zone", point)
			}
			n++
		}
	})
}

func BenchmarkClientInit(b *testing.B) {
	b.Run("main client", func(b *testing.B) {
		for b.Loop() {
			c := NewLocalTimeZone()
			_, ok := c.(*localTimeZone)
			if !ok {
				b.Errorf("cannot initialize timezone client")
			}
		}
	})
	b.Run("mock client", func(b *testing.B) {
		for b.Loop() {
			c := NewMockLocalTimeZone()
			_, ok := c.(*localTimeZone)
			if !ok {
				b.Errorf("cannot initialize timezone client")
			}
		}
	})
}

func TestNautical(t *testing.T) {
	t.Parallel()
	tt := []struct {
		lon  float64
		zone string
	}{
		{-180, "Etc/GMT+12"},
		{180, "Etc/GMT-12"},
		{-172.5, "Etc/GMT+12"},
		{172.5, "Etc/GMT-12"},
		{-172, "Etc/GMT+11"},
		{172, "Etc/GMT-11"},
		{0, "Etc/GMT"},
		{7.49, "Etc/GMT"},
		{-7.49, "Etc/GMT"},
		{7.5, "Etc/GMT-1"},
		{-7.5, "Etc/GMT+1"},
	}
	for _, tc := range tt {
		t.Run(fmt.Sprintf("%f %s", tc.lon, tc.zone), func(t *testing.T) {
			t.Parallel()
			z, _ := getNauticalZone(h3.NewLatLng(0, tc.lon))
			if z[0] != tc.zone {
				t.Errorf("expected %s got %s", tc.zone, z[0])
			}
		})
	}
}

func TestOutOfRange(t *testing.T) {
	t.Parallel()
	z := NewLocalTimeZone()
	tt := []struct {
		p   Point
		err error
	}{
		{Point{180, 0}, nil},
		{Point{-180, 0}, nil},
		{Point{0, 90}, nil},
		{Point{0, -90}, nil},
		{Point{181, 0}, ErrOutOfRange},
		{Point{-181, 0}, ErrOutOfRange},
		{Point{0, 91}, ErrOutOfRange},
		{Point{0, -91}, ErrOutOfRange},
	}
	for _, tc := range tt {
		t.Run(fmt.Sprintf("%f %f", tc.p.Lon, tc.p.Lat), func(t *testing.T) {
			t.Parallel()
			_, err := z.GetZone(tc.p)
			if err != tc.err {
				t.Errorf("expected error %v got %v", tc.err, err)
			}
		})
	}
}

func TestLoadH3Malformed(t *testing.T) {
	// Create an s2-compressed payload with invalid H3 data (bad magic)
	var buf bytes.Buffer
	buf.Write([]byte("XXXX")) // wrong magic
	buf.WriteByte(1)
	buf.WriteByte(7)
	if err := binary.Write(&buf, binary.LittleEndian, uint16(0)); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(0)); err != nil {
		t.Fatal(err)
	}

	var compressed bytes.Buffer
	s2w := s2.NewWriter(&compressed)
	if _, err := s2w.Write(buf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := s2w.Close(); err != nil {
		t.Fatal(err)
	}

	z := &localTimeZone{}
	err := z.load(compressed.Bytes())
	if err == nil {
		t.Error("expected error loading malformed H3 data")
	}
}

func TestContainsString(t *testing.T) {
	t.Parallel()
	tt := []struct {
		name     string
		s        []string
		v        string
		expected bool
	}{
		{"empty slice", []string{}, "foo", false},
		{"found at start", []string{"foo", "bar"}, "foo", true},
		{"found at end", []string{"foo", "bar"}, "bar", true},
		{"not found", []string{"foo", "bar"}, "baz", false},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := containsString(tc.s, tc.v)
			if got != tc.expected {
				t.Errorf("containsString(%v, %q) = %v; want %v", tc.s, tc.v, got, tc.expected)
			}
		})
	}
}

func TestGetZoneDeduplicatesZones(t *testing.T) {
	t.Parallel()
	// Build a synthetic cache where both a cell and its parent cell map to the same
	// timezone, verifying that getZone does not return duplicate zone entries.
	latLng := h3.NewLatLng(35.6828387, 139.7594549) // Tokyo
	resolution := 2
	cell, err := h3.LatLngToCell(latLng, resolution)
	if err != nil {
		t.Fatalf("cannot create H3 cell: %v", err)
	}
	parentCell, err := cell.Parent(resolution - 1)
	if err != nil {
		t.Fatalf("cannot get parent H3 cell: %v", err)
	}

	// Sort cell values for binary search in findCell
	c0, c1 := int64(cell), int64(parentCell)
	if c0 > c1 {
		c0, c1 = c1, c0
	}
	cache := &immutableCache{
		tzNames:    []string{"Asia/Tokyo"},
		cells:      []int64{c0, c1},
		tzIdx:      []uint16{0, 0},
		resolution: resolution,
	}

	z := &localTimeZone{}
	z.data.Store(cache)

	zones, err := z.getZone(Point{Lon: 139.7594549, Lat: 35.6828387}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(zones) != 1 {
		t.Errorf("expected 1 zone (no duplicates); got %d: %v", len(zones), zones)
	}
	if zones[0] != "Asia/Tokyo" {
		t.Errorf("expected Asia/Tokyo; got %s", zones[0])
	}
}

func TestLoadOverwrite(t *testing.T) {
	client := NewLocalTimeZone()
	c, ok := client.(*localTimeZone)
	if !ok {
		t.Errorf("cannot initialize client")
	}
	lenCells := len(c.data.Load().cells)

	if err := c.load(MockTZData); err != nil {
		t.Errorf("cannot switch client to mock data, got %v", err)
	}
	if len(c.data.Load().cells) >= lenCells {
		t.Errorf("cache not overwritten by loading new data")
	}
}
