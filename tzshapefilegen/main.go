// Code generation tool for embedding the timezone shapefile in the gotz package
// run "go generate" in the parent directory after changing the -release flag in gen.go
package main

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"

	json "github.com/json-iterator/go"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/simplify"
)

const dlURL = "https://github.com/evansiroky/timezone-boundary-builder/releases/download/%s/timezones.geojson.zip"
const versionTemplate = `// Generated by tzshapefilegen. DO NOT EDIT.

package localtimezone

// TZBoundaryVersion is the version of tzdata that was used to generate timezone boundaries
const TZBoundaryVersion = "%s"

// TZCount is the number of tzdata timezones supported
const TZCount = %d

// TZNames is an array of possible timezone names that may be returned by this library
var TZNames = []string{
%s}
`
const defaultRelease = "default"

func getMostCurrentRelease() (version string, url string, err error) {
	resp, err := http.Get("https://api.github.com/repos/evansiroky/timezone-boundary-builder/releases")
	if err != nil {
		return "", "", err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	err = resp.Body.Close()
	if err != nil {
		return "", "", err
	}

	type asset struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	}
	type release struct {
		Name   string  `json:"name"`
		Assets []asset `json:"assets"`
	}
	var response []release
	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", "", err
	}

	version = response[0].Name
	for _, asset := range response[0].Assets {
		if asset.Name != "timezones.geojson.zip" {
			continue
		}
		url = asset.BrowserDownloadURL
	}
	if url == "" {
		return "", "", fmt.Errorf("cannot find correct zip in latest timezone release")
	}
	return version, url, nil
}

func getGeoJSON(releaseURL string) ([]byte, error) {
	resp, err := http.Get(releaseURL)
	if err != nil {
		log.Fatalf("Error: could not download tz shapefile: %v\n", err)
	}

	buffer := bytes.NewBuffer([]byte{})
	_, err = io.Copy(buffer, resp.Body)
	if err != nil {
		log.Printf("Download failed: %v\n", err)
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}

	bufferReader := bytes.NewReader(buffer.Bytes())
	zipReader, err := zip.NewReader(bufferReader, resp.ContentLength)
	if err != nil {
		log.Printf("Could not access zipfile: %v\n", err)
		return nil, err
	}
	if len(zipReader.File) == 0 {
		log.Println("Error: release zip file have no files!")
		return nil, err
	} else if zipReader.File[0].Name != "combined.json" {
		log.Println("Error: first file in zip file is not combined.json")
		return nil, err
	}

	geojsonDataReader, err := zipReader.File[0].Open()
	if err != nil {
		log.Printf("Error: could not read from zip file: %v\n", err)
		return nil, err
	}

	geojsonData, err := io.ReadAll(geojsonDataReader)
	if err != nil {
		log.Printf("Error: could not read combined.json from zip file: %v\n", err)
		return nil, err
	}
	return geojsonData, nil
}

func orbExec(combinedJSON []byte) ([]byte, []string, error) {
	geojson.CustomJSONMarshaler = json.ConfigFastest
	geojson.CustomJSONUnmarshaler = json.ConfigFastest

	fc, err := geojson.UnmarshalFeatureCollection(combinedJSON)
	if err != nil {
		log.Printf("Error: could not parse combined.json: %v\n", err)
		return nil, nil, err
	}
	features := []*geojson.Feature{}
	tzNames := []string{}
	for _, feature := range fc.Features {
		tzid := feature.Properties.MustString("tzid")
		if tzid == "" {
			break
		}
		feature.Geometry = simplify.Visvalingam(0.0001, 4).Simplify(feature.Geometry)
		features = append(features, feature)
		tzNames = append(tzNames, tzid)
	}
	sort.Slice(features, func(i, j int) bool {
		return features[i].Properties.MustString("tzid") < features[j].Properties.MustString("tzid")
	})
	fc.Features = features
	reducedJSON, err := fc.MarshalJSON()
	if err != nil {
		log.Printf("Error: could not marshal reduced.json: %v\n", err)
		return nil, nil, err
	}
	tzNames = append(tzNames, "Etc/GMT")
	for offset := 1; offset <= 12; offset += 1 {
		tzNames = append(tzNames, fmt.Sprintf("Etc/GMT+%d", offset), fmt.Sprintf("Etc/GMT-%d", offset))
	}
	sort.Strings(tzNames)
	return reducedJSON, tzNames, nil
}

func generateData(geoJSON []byte) ([]byte, error) {
	buffer := bytes.NewBuffer([]byte{})
	gzipper, err := gzip.NewWriterLevel(buffer, gzip.BestCompression)
	if err != nil {
		log.Printf("Error: could not create gzip writer: %v\n", err)
		return nil, err
	}

	_, err = gzipper.Write(geoJSON)
	if err != nil {
		log.Printf("Error: could not copy data: %v\n", err)
		return nil, err
	}
	if err := gzipper.Close(); err != nil {
		log.Printf("Error: could not flush/close gzip: %v\n", err)
		return nil, err
	}

	return buffer.Bytes(), nil
}

func writeData(content []byte) error {
	err := os.WriteFile("data.json.gz", content, 0644)
	if err != nil {
		log.Printf("Error: could not write data.json.gz: %v\n", err)
		return err
	}
	return nil
}

func writeVersion(release string, tzNames []string) error {
	tzNamesFormatted := ""
	for _, tzid := range tzNames {
		tzNamesFormatted += fmt.Sprintf("	%q,\n", tzid)
	}
	content := fmt.Sprintf(versionTemplate, release, len(tzNames), tzNamesFormatted)
	outfile, err := os.Create("version.go")
	if err != nil {
		log.Printf("Error: could not create version.go: %v", err)
		return err
	}

	_, err = outfile.WriteString(content)
	if err != nil {
		log.Printf("Error: could not write content: %v", err)
		return err
	}
	err = outfile.Close()
	if err != nil {
		return err
	}
	return nil
}

func main() {
	release := flag.String("release", defaultRelease, "timezone boundary builder release version")
	flag.Parse()

	fmt.Println("*** GETTING TIMEZONE BOUNDARY RELEASE ***")
	var releaseURL string
	var err error
	if *release == defaultRelease {
		*release, releaseURL, err = getMostCurrentRelease()
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		releaseURL = fmt.Sprintf(dlURL, *release)
	}
	fmt.Printf("Downloading %s\n", releaseURL)

	fmt.Println("*** GETTING TIMEZONE BOUNDARY DATA ***")
	geojsonData, err := getGeoJSON(releaseURL)
	if err != nil {
		return
	}

	fmt.Println("*** SIMPLIFYING GEOJSON ***")
	geojsonData, tzNames, err := orbExec(geojsonData)
	if err != nil {
		return
	}
	fmt.Println("*** GEOJSON FINISHED ***")

	fmt.Println("*** GENERATING GO CODE ***")
	content, err := generateData(geojsonData)
	if err != nil {
		return
	}

	err = writeData(content)
	if err != nil {
		return
	}

	err = writeVersion(*release, tzNames)
	if err != nil {
		return
	}

	fmt.Println("*** ALL DONE, YAY ***")
}
