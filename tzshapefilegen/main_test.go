package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	localtimezone "github.com/albertyw/localtimezone/v4"
	"github.com/klauspost/compress/s2"
	"github.com/paulmach/orb"
	"github.com/uber/h3-go/v4"
)

const testGeoJSON = `{
	"type": "FeatureCollection",
	"features": [
		{
			"type": "Feature",
			"properties": {"tzid": "Test/Zone"},
			"geometry": {
				"type": "Polygon",
				"coordinates": [[[0,0],[0.2,0],[0.2,0.2],[0,0.2],[0,0]]]
			}
		}
	]
}`

// zipBytes builds an in-memory zip archive with the given file names and contents.
func zipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatalf("cannot create zip entry: %v", err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatalf("cannot write zip entry: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("cannot close zip: %v", err)
	}
	return buf.Bytes()
}

// tarGzBytes builds an in-memory gzipped tar archive with the given file names and contents.
func tarGzBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(content))}); err != nil {
			t.Fatalf("cannot write tar header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("cannot write tar entry: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("cannot close tar: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("cannot close gzip: %v", err)
	}
	return buf.Bytes()
}

// testTZDataFiles returns a minimal set of tzdata source files where
// Test/Alias is a Link to Test/Zone.
func testTZDataFiles() map[string]string {
	files := map[string]string{"NEWS": "Link Test/Zone Test/News\n"}
	for _, name := range tzdataFiles {
		files[name] = ""
	}
	files["europe"] = "Zone\tTest/Zone\t0:00\t-\tUTC\n"
	files["backward"] = "Link\tTest/Zone\tTest/Alias\n"
	return files
}

// serveTZData points tzdataURL at a test server that serves testTZDataFiles.
func serveTZData(t *testing.T) {
	t.Helper()
	server := serveBytes(t, tarGzBytes(t, testTZDataFiles()))
	tzdataURLOrig := tzdataURL
	t.Cleanup(func() { tzdataURL = tzdataURLOrig })
	tzdataURL = server.URL + "/%s"
}

// serveBytes starts a test server that responds to every request with body.
func serveBytes(t *testing.T, body []byte) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestGetMostCurrentRelease(t *testing.T) {
	version, url, err := getMostCurrentRelease()
	if err != nil {
		t.Errorf("cannot get most current timezone boundary")
	}
	if url == "" {
		t.Errorf("cannot get most current timezone url")
	}
	if version != localtimezone.TZBoundaryVersion {
		t.Errorf("timezone boundary is out of date")
	}
}

func TestGetMostCurrentReleaseRequestError(t *testing.T) {
	releasesURLOrig := releasesURL
	t.Cleanup(func() { releasesURL = releasesURLOrig })
	releasesURL = "http://127.0.0.1:0/"

	if _, _, err := getMostCurrentRelease(); err == nil {
		t.Errorf("expected error for unreachable url")
	}
}

func TestGetMostCurrentReleaseBadJSON(t *testing.T) {
	server := serveBytes(t, []byte("not json"))
	releasesURLOrig := releasesURL
	t.Cleanup(func() { releasesURL = releasesURLOrig })
	releasesURL = server.URL

	if _, _, err := getMostCurrentRelease(); err == nil {
		t.Errorf("expected error for malformed json")
	}
}

func TestGetMostCurrentReleaseNoReleases(t *testing.T) {
	server := serveBytes(t, []byte("[]"))
	releasesURLOrig := releasesURL
	t.Cleanup(func() { releasesURL = releasesURLOrig })
	releasesURL = server.URL

	_, _, err := getMostCurrentRelease()
	if err == nil || err.Error() != "no timezone releases found" {
		t.Errorf("expected no releases error, got %v", err)
	}
}

func TestGetMostCurrentReleaseNoZipAsset(t *testing.T) {
	server := serveBytes(t, []byte(`[{"name":"2024a","assets":[{"name":"other.zip","browser_download_url":"http://example.com/other.zip"}]}]`))
	releasesURLOrig := releasesURL
	t.Cleanup(func() { releasesURL = releasesURLOrig })
	releasesURL = server.URL

	_, _, err := getMostCurrentRelease()
	if err == nil || err.Error() != "cannot find correct zip in latest timezone release" {
		t.Errorf("expected missing zip error, got %v", err)
	}
}

func TestGetMostCurrentReleaseMocked(t *testing.T) {
	server := serveBytes(t, []byte(`[{"name":"2024a","assets":[{"name":"timezones.geojson.zip","browser_download_url":"http://example.com/timezones.geojson.zip"}]}]`))
	releasesURLOrig := releasesURL
	t.Cleanup(func() { releasesURL = releasesURLOrig })
	releasesURL = server.URL

	version, url, err := getMostCurrentRelease()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != "2024a" {
		t.Errorf("expected version 2024a, got %s", version)
	}
	if url != "http://example.com/timezones.geojson.zip" {
		t.Errorf("unexpected url %s", url)
	}
}

func TestGetGeoJSON(t *testing.T) {
	server := serveBytes(t, zipBytes(t, map[string]string{"combined.json": testGeoJSON}))

	data, err := getGeoJSON(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != testGeoJSON {
		t.Errorf("unexpected geojson data: %s", string(data))
	}
}

func TestGetGeoJSONRequestError(t *testing.T) {
	if _, err := getGeoJSON("http://127.0.0.1:0/"); err == nil {
		t.Errorf("expected error for unreachable url")
	}
}

func TestGetGeoJSONNotAZip(t *testing.T) {
	server := serveBytes(t, []byte("not a zip file"))

	_, err := getGeoJSON(server.URL)
	if err == nil || !strings.Contains(err.Error(), "could not access zipfile") {
		t.Errorf("expected zipfile error, got %v", err)
	}
}

func TestGetGeoJSONEmptyZip(t *testing.T) {
	server := serveBytes(t, zipBytes(t, nil))

	_, err := getGeoJSON(server.URL)
	if err == nil || err.Error() != "release zip file has no files" {
		t.Errorf("expected empty zip error, got %v", err)
	}
}

func TestGetGeoJSONWrongFileName(t *testing.T) {
	server := serveBytes(t, zipBytes(t, map[string]string{"other.json": testGeoJSON}))

	_, err := getGeoJSON(server.URL)
	if err == nil || err.Error() != "first file in zip is not combined.json" {
		t.Errorf("expected wrong file name error, got %v", err)
	}
}

func TestParseTZAliases(t *testing.T) {
	regions := []byte(`# Link Commented/Out Test/Comment
Zone	Europe/Zurich	0:34:08 -	LMT	1853 Jul 16
			1:00	Swiss	CE%sT
Zone Europe/Brussels 1:00 - CET
Link	Europe/Zurich	Europe/Busingen
Link Europe/Brussels Europe/Amsterdam
Link Europe/Old Europe/Older
Link Europe/Zurich Europe/Old
Link Nowhere/Zone Test/Dangling
`)
	backzone := []byte(`Zone Europe/Amsterdam 1:00 - CET
Link Europe/Amsterdam Europe/Busingen
`)
	want := map[string]string{
		"Europe/Busingen": "Europe/Amsterdam",
		"Europe/Old":      "Europe/Zurich",
		"Europe/Older":    "Europe/Zurich",
		"Test/Dangling":   "Nowhere/Zone",
	}

	got := parseTZAliases([][]byte{regions, backzone})
	if !maps.Equal(got, want) {
		t.Errorf("unexpected aliases %v, want %v", got, want)
	}
}

func TestParseTZAliasesCycle(t *testing.T) {
	got := parseTZAliases([][]byte{[]byte("Link Test/A Test/B\nLink Test/B Test/A\n")})
	if len(got) != 2 {
		t.Errorf("expected cyclic links to terminate, got %v", got)
	}
}

func TestCanonicalName(t *testing.T) {
	aliases := map[string]string{"Test/Alias": "Test/Zone"}
	if got := canonicalName(aliases, "Test/Alias"); got != "Test/Zone" {
		t.Errorf("expected Test/Zone, got %s", got)
	}
	if got := canonicalName(aliases, "Test/Other"); got != "Test/Other" {
		t.Errorf("expected Test/Other, got %s", got)
	}
}

func TestGetTZAliases(t *testing.T) {
	serveTZData(t)

	aliases, err := getTZAliases("2024a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{"Test/Alias": "Test/Zone"}
	if !maps.Equal(aliases, want) {
		t.Errorf("unexpected aliases %v, want %v", aliases, want)
	}
}

func TestGetTZAliasesURL(t *testing.T) {
	var path string
	body := tarGzBytes(t, testTZDataFiles())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	tzdataURLOrig := tzdataURL
	t.Cleanup(func() { tzdataURL = tzdataURLOrig })
	tzdataURL = server.URL + "/tzdata%s.tar.gz"

	if _, err := getTZAliases("2024a"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "/tzdata2024a.tar.gz" {
		t.Errorf("unexpected request path %s", path)
	}
}

func TestGetTZAliasesRequestError(t *testing.T) {
	tzdataURLOrig := tzdataURL
	t.Cleanup(func() { tzdataURL = tzdataURLOrig })
	tzdataURL = "http://127.0.0.1:0/%s"

	_, err := getTZAliases("2024a")
	if err == nil || !strings.Contains(err.Error(), "could not download tzdata") {
		t.Errorf("expected download error, got %v", err)
	}
}

func TestGetTZAliasesNotFound(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)
	tzdataURLOrig := tzdataURL
	t.Cleanup(func() { tzdataURL = tzdataURLOrig })
	tzdataURL = server.URL + "/%s"

	_, err := getTZAliases("2024a")
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("expected not found error, got %v", err)
	}
}

func TestGetTZAliasesNotGzip(t *testing.T) {
	server := serveBytes(t, []byte("not gzip"))
	tzdataURLOrig := tzdataURL
	t.Cleanup(func() { tzdataURL = tzdataURLOrig })
	tzdataURL = server.URL + "/%s"

	_, err := getTZAliases("2024a")
	if err == nil || !strings.Contains(err.Error(), "could not decompress tzdata") {
		t.Errorf("expected decompress error, got %v", err)
	}
}

func TestGetTZAliasesNotTar(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, _ = gw.Write(bytes.Repeat([]byte("x"), 1024))
	_ = gw.Close()
	server := serveBytes(t, buf.Bytes())
	tzdataURLOrig := tzdataURL
	t.Cleanup(func() { tzdataURL = tzdataURLOrig })
	tzdataURL = server.URL + "/%s"

	_, err := getTZAliases("2024a")
	if err == nil || !strings.Contains(err.Error(), "could not read tzdata archive") {
		t.Errorf("expected archive error, got %v", err)
	}
}

func TestGetTZAliasesTruncated(t *testing.T) {
	files := testTZDataFiles()
	files["europe"] = strings.Repeat("Zone Test/Zone 0:00 - UTC\n", 1000)
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	if err := tw.WriteHeader(&tar.Header{Name: "europe", Mode: 0644, Size: int64(len(files["europe"]))}); err != nil {
		t.Fatalf("cannot write tar header: %v", err)
	}
	_, _ = tw.Write([]byte(files["europe"]))
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, _ = gw.Write(tarBuf.Bytes()[:1024])
	_ = gw.Close()
	server := serveBytes(t, buf.Bytes())
	tzdataURLOrig := tzdataURL
	t.Cleanup(func() { tzdataURL = tzdataURLOrig })
	tzdataURL = server.URL + "/%s"

	_, err := getTZAliases("2024a")
	if err == nil || !strings.Contains(err.Error(), "could not read tzdata file europe") {
		t.Errorf("expected file read error, got %v", err)
	}
}

func TestGetTZAliasesMissingFile(t *testing.T) {
	files := testTZDataFiles()
	delete(files, "backzone")
	server := serveBytes(t, tarGzBytes(t, files))
	tzdataURLOrig := tzdataURL
	t.Cleanup(func() { tzdataURL = tzdataURLOrig })
	tzdataURL = server.URL + "/%s"

	_, err := getTZAliases("2024a")
	if err == nil || err.Error() != "tzdata archive is missing backzone" {
		t.Errorf("expected missing file error, got %v", err)
	}
}

func TestOrbPolygonToH3Empty(t *testing.T) {
	geoPolygon := orbPolygonToH3(orb.Polygon{})
	if len(geoPolygon.GeoLoop) != 0 || len(geoPolygon.Holes) != 0 {
		t.Errorf("expected empty geopolygon, got %v", geoPolygon)
	}
}

func TestOrbPolygonToH3Outer(t *testing.T) {
	polygon := orb.Polygon{{{1, 2}, {3, 4}, {5, 6}}}
	geoPolygon := orbPolygonToH3(polygon)
	if len(geoPolygon.GeoLoop) != 3 {
		t.Fatalf("expected 3 points, got %d", len(geoPolygon.GeoLoop))
	}
	if len(geoPolygon.Holes) != 0 {
		t.Errorf("expected no holes, got %d", len(geoPolygon.Holes))
	}
	// orb stores [lon, lat] while h3 stores (lat, lng)
	expected := h3.NewLatLng(2, 1)
	if geoPolygon.GeoLoop[0] != expected {
		t.Errorf("expected %v, got %v", expected, geoPolygon.GeoLoop[0])
	}
}

func TestOrbPolygonToH3Holes(t *testing.T) {
	polygon := orb.Polygon{
		{{0, 0}, {10, 0}, {10, 10}, {0, 10}},
		{{1, 1}, {2, 1}, {2, 2}},
	}
	geoPolygon := orbPolygonToH3(polygon)
	if len(geoPolygon.GeoLoop) != 4 {
		t.Errorf("expected 4 outer points, got %d", len(geoPolygon.GeoLoop))
	}
	if len(geoPolygon.Holes) != 1 {
		t.Fatalf("expected 1 hole, got %d", len(geoPolygon.Holes))
	}
	if len(geoPolygon.Holes[0]) != 3 {
		t.Errorf("expected 3 hole points, got %d", len(geoPolygon.Holes[0]))
	}
}

// parseH3Data reads back the binary format produced by orbExec.
func parseH3Data(t *testing.T, data []byte) (names []string, entries int) {
	t.Helper()
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
	entries = int(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4
	if len(data) != offset+entries*entrySize {
		t.Errorf("unexpected data length %d, want %d", len(data), offset+entries*entrySize)
	}
	return names, entries
}

func TestOrbExec(t *testing.T) {
	data, tzNames, err := orbExec([]byte(testGeoJSON), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	names, entries := parseH3Data(t, data)
	if len(names) != 1 || names[0] != "Test/Zone" {
		t.Errorf("unexpected string table %v", names)
	}
	if entries == 0 {
		t.Errorf("expected at least one cell entry")
	}

	// The returned names include the timezone plus the 25 nautical zones
	if len(tzNames) != 26 {
		t.Fatalf("expected 26 timezone names, got %d", len(tzNames))
	}
	found := map[string]bool{}
	for _, name := range tzNames {
		found[name] = true
	}
	for _, name := range []string{"Test/Zone", "Etc/GMT", "Etc/GMT+12", "Etc/GMT-12"} {
		if !found[name] {
			t.Errorf("expected %s in timezone names", name)
		}
	}
	if !slices.IsSorted(tzNames) {
		t.Errorf("expected timezone names to be sorted, got %v", tzNames)
	}
}

func TestOrbExecMultiPolygon(t *testing.T) {
	geoJSON := `{
		"type": "FeatureCollection",
		"features": [
			{
				"type": "Feature",
				"properties": {"tzid": "Test/Multi"},
				"geometry": {
					"type": "MultiPolygon",
					"coordinates": [
						[[[0,0],[0.2,0],[0.2,0.2],[0,0.2],[0,0]]],
						[[[10,10],[10.2,10],[10.2,10.2],[10,10.2],[10,10]]]
					]
				}
			}
		]
	}`
	data, tzNames, err := orbExec([]byte(geoJSON), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names, entries := parseH3Data(t, data)
	if len(names) != 1 || names[0] != "Test/Multi" {
		t.Errorf("unexpected string table %v", names)
	}
	if entries == 0 {
		t.Errorf("expected at least one cell entry")
	}
	if len(tzNames) != 26 {
		t.Errorf("expected 26 timezone names, got %d", len(tzNames))
	}
}

func TestOrbExecDuplicateTimezone(t *testing.T) {
	geoJSON := `{
		"type": "FeatureCollection",
		"features": [
			{
				"type": "Feature",
				"properties": {"tzid": "Test/Zone"},
				"geometry": {"type": "Polygon", "coordinates": [[[0,0],[0.2,0],[0.2,0.2],[0,0.2],[0,0]]]}
			},
			{
				"type": "Feature",
				"properties": {"tzid": "Test/Zone"},
				"geometry": {"type": "Polygon", "coordinates": [[[0,0],[0.2,0],[0.2,0.2],[0,0.2],[0,0]]]}
			}
		]
	}`
	data, _, err := orbExec([]byte(geoJSON), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names, _ := parseH3Data(t, data)
	if len(names) != 1 {
		t.Errorf("expected deduplicated string table, got %v", names)
	}
}

func TestOrbExecAlias(t *testing.T) {
	geoJSON := `{
		"type": "FeatureCollection",
		"features": [
			{
				"type": "Feature",
				"properties": {"tzid": "Test/Alias"},
				"geometry": {"type": "Polygon", "coordinates": [[[10,10],[10.2,10],[10.2,10.2],[10,10.2],[10,10]]]}
			},
			{
				"type": "Feature",
				"properties": {"tzid": "Test/Zone"},
				"geometry": {"type": "Polygon", "coordinates": [[[0,0],[0.2,0],[0.2,0.2],[0,0.2],[0,0]]]}
			}
		]
	}`
	zoneOnly, _, err := orbExec([]byte(testGeoJSON), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, zoneEntries := parseH3Data(t, zoneOnly)

	data, tzNames, err := orbExec([]byte(geoJSON), map[string]string{"Test/Alias": "Test/Zone"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names, entries := parseH3Data(t, data)
	if len(names) != 1 || names[0] != "Test/Zone" {
		t.Errorf("expected alias to be canonicalized, got string table %v", names)
	}
	if entries <= zoneEntries {
		t.Errorf("expected alias cells to merge into canonical zone, got %d entries", entries)
	}
	if slices.Contains(tzNames, "Test/Alias") || !slices.Contains(tzNames, "Test/Zone") {
		t.Errorf("expected only canonical name in timezone names, got %v", tzNames)
	}
}

func TestOrbExecUnsupportedGeometry(t *testing.T) {
	geoJSON := `{
		"type": "FeatureCollection",
		"features": [
			{
				"type": "Feature",
				"properties": {"tzid": "Test/Point"},
				"geometry": {"type": "Point", "coordinates": [0,0]}
			}
		]
	}`
	data, tzNames, err := orbExec([]byte(geoJSON), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names, entries := parseH3Data(t, data)
	if len(names) != 1 || names[0] != "Test/Point" {
		t.Errorf("unexpected string table %v", names)
	}
	if entries != 0 {
		t.Errorf("expected no cell entries, got %d", entries)
	}
	if len(tzNames) != 26 {
		t.Errorf("expected 26 timezone names, got %d", len(tzNames))
	}
}

func TestOrbExecMissingTZID(t *testing.T) {
	geoJSON := `{
		"type": "FeatureCollection",
		"features": [
			{
				"type": "Feature",
				"properties": {},
				"geometry": {"type": "Polygon", "coordinates": [[[0,0],[0.2,0],[0.2,0.2],[0,0.2],[0,0]]]}
			}
		]
	}`
	data, tzNames, err := orbExec([]byte(geoJSON), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names, entries := parseH3Data(t, data)
	if len(names) != 0 {
		t.Errorf("expected empty string table, got %v", names)
	}
	if entries != 0 {
		t.Errorf("expected no cell entries, got %d", entries)
	}
	if len(tzNames) != 25 {
		t.Errorf("expected 25 nautical timezone names, got %d", len(tzNames))
	}
}

func TestOrbExecBadJSON(t *testing.T) {
	_, _, err := orbExec([]byte("not json"), nil)
	if err == nil || !strings.Contains(err.Error(), "could not parse combined.json") {
		t.Errorf("expected parse error, got %v", err)
	}
}

func TestGenerateData(t *testing.T) {
	data := []byte("some timezone data")
	compressed, err := generateData(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decompressed, err := s2.Decode(nil, compressed)
	if err != nil {
		t.Fatalf("cannot decompress: %v", err)
	}
	if !bytes.Equal(decompressed, data) {
		t.Errorf("expected %q, got %q", data, decompressed)
	}
}

func TestWriteData(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := writeData([]byte("data")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	content, err := os.ReadFile("data.h3.s2")
	if err != nil {
		t.Fatalf("cannot read written file: %v", err)
	}
	if string(content) != "data" {
		t.Errorf("unexpected content %q", string(content))
	}
}

func TestWriteDataError(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.Mkdir("data.h3.s2", 0755); err != nil {
		t.Fatalf("cannot create blocking directory: %v", err)
	}

	err := writeData([]byte("data"))
	if err == nil || !strings.Contains(err.Error(), "could not write data.h3.s2") {
		t.Errorf("expected write error, got %v", err)
	}
}

func TestWriteVersion(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := writeVersion("2024a", []string{"Etc/GMT", "Test/Zone"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	content, err := os.ReadFile("version.go")
	if err != nil {
		t.Fatalf("cannot read written file: %v", err)
	}
	for _, want := range []string{
		`const TZBoundaryVersion = "2024a"`,
		`const TZCount = 2`,
		"\t\"Etc/GMT\",\n\t\"Test/Zone\",\n",
	} {
		if !strings.Contains(string(content), want) {
			t.Errorf("expected %q in version.go, got:\n%s", want, string(content))
		}
	}
}

func TestWriteVersionError(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("version.go", 0755); err != nil {
		t.Fatalf("cannot create blocking directory: %v", err)
	}

	err := writeVersion("2024a", nil)
	if err == nil || !strings.Contains(err.Error(), "could not write version.go") {
		t.Errorf("expected write error, got %v", err)
	}
}

func TestRun(t *testing.T) {
	server := serveBytes(t, zipBytes(t, map[string]string{"combined.json": testGeoJSON}))
	dlURLOrig := dlURL
	t.Cleanup(func() { dlURL = dlURLOrig })
	dlURL = server.URL + "/%s"
	serveTZData(t)
	t.Chdir(t.TempDir())

	if err := run("2024a"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile("data.h3.s2")
	if err != nil {
		t.Fatalf("cannot read data.h3.s2: %v", err)
	}
	decompressed, err := s2.Decode(nil, data)
	if err != nil {
		t.Fatalf("cannot decompress data.h3.s2: %v", err)
	}
	names, entries := parseH3Data(t, decompressed)
	if len(names) != 1 || names[0] != "Test/Zone" {
		t.Errorf("unexpected string table %v", names)
	}
	if entries == 0 {
		t.Errorf("expected at least one cell entry")
	}

	version, err := os.ReadFile("version.go")
	if err != nil {
		t.Fatalf("cannot read version.go: %v", err)
	}
	if !strings.Contains(string(version), `const TZBoundaryVersion = "2024a"`) {
		t.Errorf("unexpected version.go content:\n%s", string(version))
	}
}

func TestRunDefaultRelease(t *testing.T) {
	zipData := zipBytes(t, map[string]string{"combined.json": testGeoJSON})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/releases" {
			_, _ = w.Write([]byte(`[{"name":"2024a","assets":[{"name":"timezones.geojson.zip","browser_download_url":"` + "http://" + r.Host + `/zip"}]}]`))
			return
		}
		_, _ = w.Write(zipData)
	}))
	t.Cleanup(server.Close)

	releasesURLOrig := releasesURL
	t.Cleanup(func() { releasesURL = releasesURLOrig })
	releasesURL = server.URL + "/releases"
	serveTZData(t)
	t.Chdir(t.TempDir())

	if err := run(defaultRelease); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	version, err := os.ReadFile("version.go")
	if err != nil {
		t.Fatalf("cannot read version.go: %v", err)
	}
	if !strings.Contains(string(version), `const TZBoundaryVersion = "2024a"`) {
		t.Errorf("unexpected version.go content:\n%s", string(version))
	}
}

func TestRunReleaseLookupError(t *testing.T) {
	releasesURLOrig := releasesURL
	t.Cleanup(func() { releasesURL = releasesURLOrig })
	releasesURL = "http://127.0.0.1:0/"

	if err := run(defaultRelease); err == nil {
		t.Errorf("expected error for unreachable releases url")
	}
}

func TestRunDownloadError(t *testing.T) {
	dlURLOrig := dlURL
	t.Cleanup(func() { dlURL = dlURLOrig })
	dlURL = "http://127.0.0.1:0/%s"

	if err := run("2024a"); err == nil {
		t.Errorf("expected error for unreachable download url")
	}
}

func TestRunTZDataError(t *testing.T) {
	server := serveBytes(t, zipBytes(t, map[string]string{"combined.json": testGeoJSON}))
	dlURLOrig := dlURL
	t.Cleanup(func() { dlURL = dlURLOrig })
	dlURL = server.URL + "/%s"
	tzdataURLOrig := tzdataURL
	t.Cleanup(func() { tzdataURL = tzdataURLOrig })
	tzdataURL = "http://127.0.0.1:0/%s"

	err := run("2024a")
	if err == nil || !strings.Contains(err.Error(), "could not download tzdata") {
		t.Errorf("expected tzdata error, got %v", err)
	}
}

func TestRunBadGeoJSON(t *testing.T) {
	server := serveBytes(t, zipBytes(t, map[string]string{"combined.json": "not json"}))
	dlURLOrig := dlURL
	t.Cleanup(func() { dlURL = dlURLOrig })
	dlURL = server.URL + "/%s"
	serveTZData(t)

	if err := run("2024a"); err == nil {
		t.Errorf("expected error for malformed combined.json")
	}
}

func TestRunWriteError(t *testing.T) {
	server := serveBytes(t, zipBytes(t, map[string]string{"combined.json": testGeoJSON}))
	dlURLOrig := dlURL
	t.Cleanup(func() { dlURL = dlURLOrig })
	dlURL = server.URL + "/%s"
	serveTZData(t)
	t.Chdir(t.TempDir())
	if err := os.Mkdir("data.h3.s2", 0755); err != nil {
		t.Fatalf("cannot create blocking directory: %v", err)
	}

	err := run("2024a")
	if err == nil || !strings.Contains(err.Error(), "could not write data.h3.s2") {
		t.Errorf("expected write error, got %v", err)
	}
}

func TestRunWriteVersionError(t *testing.T) {
	server := serveBytes(t, zipBytes(t, map[string]string{"combined.json": testGeoJSON}))
	dlURLOrig := dlURL
	t.Cleanup(func() { dlURL = dlURLOrig })
	dlURL = server.URL + "/%s"
	serveTZData(t)
	t.Chdir(t.TempDir())
	if err := os.Mkdir("version.go", 0755); err != nil {
		t.Fatalf("cannot create blocking directory: %v", err)
	}

	err := run("2024a")
	if err == nil || !strings.Contains(err.Error(), "could not write version.go") {
		t.Errorf("expected write error, got %v", err)
	}
}
