package localtimezone

import (
	"testing"
)

var testcases = []struct {
	name  string
	point Point
	zones []string
}{
	{
		"Riga",
		Point{24.105078, 56.946285},
		[]string{"Europe/Riga"},
	},
	{
		"Tokyo",
		Point{139.7594549, 35.6828387},
		[]string{"Asia/Tokyo"},
	},
	{
		"Urumqi/Shanghai",
		Point{87.319461, 43.419754},
		[]string{"Asia/Shanghai", "Asia/Urumqi"},
	},
	{
		"Tuvalu",
		Point{178.1167698, -7.768959},
		[]string{"Pacific/Funafuti"},
	},
	{
		"Baker Island",
		Point{-176.474331436, 0.190165906},
		[]string{"Etc/GMT+12"},
	},
	{
		"Asuncion",
		Point{-57.637517, -25.335772},
		[]string{"America/Asuncion"},
	},
	{
		"Across the river from Asuncion",
		Point{-57.681572, -25.351069},
		[]string{"America/Argentina/Cordoba"},
	},
	{
		"Singapore",
		Point{103.811988, 1.466482},
		[]string{"Asia/Singapore"},
	},
	{
		"Across the river from Singapore",
		Point{103.768481, 1.462410},
		[]string{"Asia/Kuala_Lumpur"},
	},
}

func TestData(t *testing.T) {
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			tzid, err := GetZone(tc.point)
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
