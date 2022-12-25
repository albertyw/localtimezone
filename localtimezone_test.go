package localtimezone

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"sync"
	"testing"

	"github.com/paulmach/orb"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestPointFromOrb(t *testing.T) {
	p1 := orb.Point{1, 2}
	p2 := pointFromOrb(p1)
	if p2.Lon != p1[0] {
		t.Errorf("expected point longitude %v; got %v", p1[0], p2.Lon)
	}
	if p2.Lat != p1[1] {
		t.Errorf("expected point latitude %v; got %v", p1[1], p2.Lat)
	}
}

func TestPointToOrb(t *testing.T) {
	p1 := Point{Lon: 1, Lat: 2}
	p2 := pointToOrb(p1)
	if p2[0] != p1.Lon {
		t.Errorf("expected point longitude %v; got %v", p1.Lon, p2[0])
	}
	if p2[1] != p1.Lat {
		t.Errorf("expected point latitude %v; got %v", p1.Lat, p2[1])
	}
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

	shapeFile := []byte("asdf")
	err = c.load(shapeFile)
	if err == nil {
		t.Errorf("expected error when loading malformed data")
	}

	shapeFile2 := bytes.NewBufferString("")
	writer := gzip.NewWriter(shapeFile2)
	_, err = writer.Write([]byte("asdf"))
	if err != nil {
		t.Errorf("cannot write to gzip, got error %v", err)
	}
	err = c.load(shapeFile2.Bytes())
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
		"Tuvalu",
		Point{178.1167698, -7.768959},
		result{
			zones: []string{"Pacific/Funafuti"},
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
}

func TestGetZone(t *testing.T) {
	t.Parallel()
	z, err := NewLocalTimeZone()
	if err != nil {
		t.Errorf("cannot initialize timezone client: %v", err)
	}
	for _, tc := range tt {
		tc := tc // Remove race condition over test fields
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tzid, err := z.GetZone(tc.point)
			if err != tc.err {
				t.Errorf("expected err %v; got %v", tc.err, err)
			}
			if len(tzid) != len(tc.zones) {
				t.Errorf("expected %d zones; got %d", len(tc.zones), len(tzid))
			}
			for i, zone := range tc.zones {
				if tzid[i] != zone {
					t.Errorf("expected zone %s; got %s", zone, tzid[i])
				}
			}
		})
	}
}

func TestMockLocalTimeZone(t *testing.T) {
	z := NewMockLocalTimeZone()
	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tzid, err := z.GetZone(tc.point)
			if err != tc.err {
				t.Errorf("expected err %v; got %v", tc.err, err)
			}
			if len(tzid) != 1 {
				t.Errorf("expected 1 zone; got %d", len(tzid))
			}
			if tzid[0] != MockTimeZone {
				t.Errorf("expected zone America/Los_Angeles; got %s", tzid[0])
			}
		})
	}
}

func TestMockLocalTimeZonePanic(t *testing.T) {
	tempMockTZShapeFile := MockTZShapeFile
	MockTZShapeFile = []byte("asdf")
	defer func() {
		MockTZShapeFile = tempMockTZShapeFile
		if r := recover(); r == nil {
			t.Errorf("expected a panic; got no panic")
		}
	}()
	NewMockLocalTimeZone()
}

func BenchmarkZones(b *testing.B) {
	zInterface, err := NewLocalTimeZone()
	if err != nil {
		b.Errorf("cannot initialize timezone client: %v", err)
	}
	z, ok := zInterface.(*localTimeZone)
	if !ok {
		b.Errorf("cannot initialize timezone client")
	}
	z.mu.RLock()
	z.mu.RUnlock() //lint:ignore SA2001 Make sure client has loaded
	b.Run("polygon centers", func(b *testing.B) {
	Loop:
		for n := 0; n < b.N; {
			for _, v := range *z.centerCache {
				for i := range v {
					if n > b.N {
						break Loop
					}
					_, err := z.GetZone(pointFromOrb(v[i]))
					if err != nil {
						b.Errorf("point %v did not return a zone", v[i])
					}
					n++
				}
			}
		}
	})
	b.Run("test cases", func(b *testing.B) {
	Loop:
		for n := 0; n < b.N; {
			for _, tc := range tt {
				if n > b.N {
					break Loop
				}
				_, err := z.GetZone(tc.point)
				if err != nil {
					b.Errorf("point %v did not return a zone", tc.point)
				}
				n++
			}

		}
	})
}

func BenchmarkClientInit(b *testing.B) {
	b.Run("main client", func(b *testing.B) {
		for n := 0; n < b.N; {
			c, err := NewLocalTimeZone()
			if err != nil {
				b.Errorf("client could not initialize because of %v", err)
			}
			cStruct, ok := c.(*localTimeZone)
			if !ok {
				b.Errorf("cannot initialize timezone client")
			}
			cStruct.mu.RLock()
			cStruct.mu.RUnlock() //lint:ignore SA2001 Wait for the client to load
			n++
		}
	})
	b.Run("mock client", func(b *testing.B) {
		for n := 0; n < b.N; {
			c := NewMockLocalTimeZone()
			cStruct, ok := c.(*localTimeZone)
			if !ok {
				b.Errorf("cannot initialize timezone client")
			}
			cStruct.mu.RLock()
			cStruct.mu.RUnlock() //lint:ignore SA2001 Wait for the client to load
			n++
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
		tc := tc // Remove race condition over test fields
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
		tc := tc // Remove race condition over test fields
		t.Run(fmt.Sprintf("%f %f", tc.p.Lon, tc.p.Lat), func(t *testing.T) {
			t.Parallel()
			_, err := z.GetZone(tc.p)
			if err != tc.err {
				t.Errorf("expected error %v got %v", tc.err, err)
			}
		})
	}
}

func TestLoadGeoJSONMalformed(t *testing.T) {
	data := "{"
	reader := bytes.NewBufferString(data)
	client, err := NewLocalTimeZone()
	if err != nil {
		t.Errorf("cannot initialize client, got %v", err)
	}
	c, ok := client.(*localTimeZone)
	if !ok {
		t.Errorf("cannot initialize client")
	}
	err = c.LoadGeoJSON(reader)
	if err == nil {
		t.Errorf("expected error, got %v", err)
	}
	unlocked := c.mu.TryLock()
	if !unlocked {
		t.Errorf("expected lock to be released")
	}
	defer c.mu.Unlock()

	if len(c.orbData.Features) != 0 {
		t.Errorf("orbData not reset")
	}
	if len(c.boundCache) != 0 {
		t.Errorf("boundCache not reset")
	}
	if len(*c.centerCache) != 0 {
		t.Errorf("centerCache not reset")
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
	c.mu.RLock()
	lenOrbData := len(c.orbData.Features)
	lenBoundCache := len(c.boundCache)
	lenCenterCache := len(*c.centerCache)
	c.mu.RUnlock()

	err = c.load(MockTZShapeFile)
	c.mu.RLock()
	defer c.mu.RUnlock()
	if err != nil {
		t.Errorf("cannot switch client to mock data, got %v", err)
	}
	if len(c.orbData.Features) >= lenOrbData {
		t.Errorf("orbData not overwritten by loading new data")
	}
	if len(c.boundCache) >= lenBoundCache {
		t.Errorf("boundCache not overwritten by loading new data")
	}
	if len(*c.centerCache) >= lenCenterCache {
		t.Errorf("centerCache not overwritten by loading new data")
	}
}
