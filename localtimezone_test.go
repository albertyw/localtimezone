package localtimezone

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"
	"testing"

	"github.com/klauspost/compress/s2"
	"github.com/paulmach/orb"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestLoadError(t *testing.T) {
	client, err := NewLocalTimeZone()
	if err != nil {
		t.Errorf("error when initializing client: %v", err)
	}
	c, ok := client.(*localTimeZone)
	if !ok {
		t.Errorf("error when initializing client")
	}

	badData := []byte("asdf")
	err = c.load(badData)
	if err == nil {
		t.Errorf("expected error when loading malformed data")
	}

	var badData2 bytes.Buffer
	writer := s2.NewWriter(&badData2)
	_, err = writer.Write([]byte("asdf"))
	if err != nil {
		t.Errorf("cannot write to s2, got error %v", err)
	}
	if err = writer.Close(); err != nil {
		t.Errorf("cannot close s2 writer, got error %v", err)
	}
	err = c.load(badData2.Bytes())
	if err == nil {
		t.Errorf("expected error when loading malformed data")
	}
}

func TestParallelNewLocalTimeZone(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := NewLocalTimeZone()
			if err != nil {
				t.Errorf("error when initializing client: %v", err)
			}
		}()
	}
	wg.Wait()
}

type result struct {
	zones []string
	err   error
}

var tt = []struct {
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
	z, err := NewLocalTimeZone()
	if err != nil {
		t.Errorf("cannot initialize timezone client: %v", err)
	}
	for _, tc := range tt {
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
	z, err := NewLocalTimeZone()
	if err != nil {
		t.Errorf("cannot initialize timezone client: %v", err)
	}
	for _, tc := range tt {
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
	for _, tc := range tt {
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
	z, err := NewLocalTimeZone()
	if err != nil {
		b.Errorf("cannot initialize timezone client: %v", err)
	}

	// Ensure client has finished loading data
	_, err = z.GetZone(Point{0, 0})
	if err != nil {
		b.Errorf("cannot initialize timezone client: %v", err)
	}

	b.Run("test cases", func(b *testing.B) {
		points := make([]Point, 0, len(tt))
		for _, tc := range tt {
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
			c, err := NewLocalTimeZone()
			if err != nil {
				b.Errorf("client could not initialize because of %v", err)
			}
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
			z, _ := getNauticalZone(orb.Point{tc.lon, 0})
			if z[0] != tc.zone {
				t.Errorf("expected %s got %s", tc.zone, z[0])
			}
		})
	}
}

func TestOutOfRange(t *testing.T) {
	t.Parallel()
	z, err := NewLocalTimeZone()
	if err != nil {
		t.Errorf("cannot initialize timezone client: %v", err)
	}
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

func TestLoadOverwrite(t *testing.T) {
	client, err := NewLocalTimeZone()
	if err != nil {
		t.Errorf("cannot initialize client, got %v", err)
	}
	c, ok := client.(*localTimeZone)
	if !ok {
		t.Errorf("cannot initialize client")
	}
	lenCells := len(c.data.Load().cells)

	err = c.load(MockTZData)
	if err != nil {
		t.Errorf("cannot switch client to mock data, got %v", err)
	}
	if len(c.data.Load().cells) >= lenCells {
		t.Errorf("cache not overwritten by loading new data")
	}
}
