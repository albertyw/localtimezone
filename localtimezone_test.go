package localtimezone

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"sync"
	"testing"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/encoding/wkb"
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

	// Ensure client has finished loading data
	_, err = z.GetZone(Point{0, 0})
	if err != nil {
		b.Errorf("cannot initialize timezone client: %v", err)
	}
	cache := z.data.Load()

	b.Run("polygon centers", func(b *testing.B) {
		centers := []orb.Point{}
		for _, d := range cache.tzData {
			centers = append(centers, d.centers...)
		}
		n := 0
		for b.Loop() {
			cs := centers[n%len(centers)]
			_, err := z.GetZone(pointFromOrb(cs))
			if err != nil {
				b.Errorf("point %v did not return a zone", centers)
			}
			n++
		}
	})
	b.Run("test cases", func(b *testing.B) {
		points := make([]Point, len(tt))
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
	if len(c.data.Load().tzData) != 0 {
		t.Errorf("tzData not reset")
	}
}

func TestWKBRoundTrip(t *testing.T) {
	t.Parallel()
	polygon := orb.Polygon{
		orb.Ring{
			{-180, -90},
			{180, -90},
			{180, 90},
			{-180, 90},
			{-180, -90},
		},
	}
	wkbBytes, err := wkb.Marshal(polygon)
	if err != nil {
		t.Fatalf("wkb.Marshal: %v", err)
	}
	geometry, err := wkb.Unmarshal(wkbBytes)
	if err != nil {
		t.Fatalf("wkb.Unmarshal: %v", err)
	}
	result, ok := geometry.(orb.Polygon)
	if !ok {
		t.Fatalf("expected orb.Polygon, got %T", geometry)
	}
	if len(result) != len(polygon) {
		t.Fatalf("expected %d rings, got %d", len(polygon), len(result))
	}
	for i, ring := range polygon {
		if len(result[i]) != len(ring) {
			t.Fatalf("ring %d: expected %d points, got %d", i, len(ring), len(result[i]))
		}
		for j, pt := range ring {
			if result[i][j] != pt {
				t.Errorf("ring %d point %d: expected %v, got %v", i, j, pt, result[i][j])
			}
		}
	}
}

func TestLoadGeoJSONValid(t *testing.T) {
	geojsonData := `{
		"type": "FeatureCollection",
		"features": [{
			"type": "Feature",
			"properties": {"tzid": "America/New_York"},
			"geometry": {
				"type": "Polygon",
				"coordinates": [[[-80,35],[-75,35],[-75,40],[-80,40],[-80,35]]]
			}
		}]
	}`
	reader := bytes.NewBufferString(geojsonData)
	z := &localTimeZone{}
	// Initialize with empty cache so LoadGeoJSON can overwrite
	cache := immutableCache{tzData: []tzData{}}
	z.data.Store(&cache)
	err := z.LoadGeoJSON(reader)
	if err != nil {
		t.Fatalf("LoadGeoJSON: %v", err)
	}
	tzids, err := z.GetZone(Point{Lon: -77.5, Lat: 37.5})
	if err != nil {
		t.Fatalf("GetZone: %v", err)
	}
	if len(tzids) != 1 || tzids[0] != "America/New_York" {
		t.Errorf("expected [America/New_York], got %v", tzids)
	}
}

func TestLoadWKBMalformed(t *testing.T) {
	// Create a gzipped payload with invalid WKB data
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, uint32(1)); err != nil {
		t.Fatal(err)
	}
	tzid := []byte("Test/Zone")
	if err := binary.Write(&buf, binary.LittleEndian, uint16(len(tzid))); err != nil {
		t.Fatal(err)
	}
	buf.Write(tzid)
	if err := binary.Write(&buf, binary.LittleEndian, uint32(3)); err != nil {
		t.Fatal(err)
	}
	buf.Write([]byte{0, 0, 0}) // invalid WKB

	var compressed bytes.Buffer
	gzipper := gzip.NewWriter(&compressed)
	if _, err := gzipper.Write(buf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := gzipper.Close(); err != nil {
		t.Fatal(err)
	}

	z := &localTimeZone{}
	err := z.load(compressed.Bytes())
	if err == nil {
		t.Error("expected error loading malformed WKB data")
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
	lenTzData := len(c.data.Load().tzData)

	err = c.load(MockTZShapeFile)
	if err != nil {
		t.Errorf("cannot switch client to mock data, got %v", err)
	}
	if len(c.data.Load().tzData) >= lenTzData {
		t.Errorf("boundCache not overwritten by loading new data")
	}
}
