// Code generation tool for embedding the timezone shapefile in the gotz package
// run "go generate" in the parent directory after changing the -release flag in gen.go
// You need mapshaper to be installed and it must be in your $PATH
// More info on mapshaper: https://github.com/mbloch/mapshaper
package main

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"

	"github.com/goccy/go-json"
)

const dlURL = "https://github.com/evansiroky/timezone-boundary-builder/releases/download/%s/timezones.geojson.zip"
const dataTemplate = `// Generated by tzshapefilegen. DO NOT EDIT.

package data

var TZShapeFile = []byte("%s")
`
const versionTemplate = `// Generated by tzshapefilegen. DO NOT EDIT.

package localtimezone

// TZBoundaryVersion is the version of tzdata that was used to generate timezone boundaries
const TZBoundaryVersion = "%s"
`
const defaultRelease = "default"

func getMostCurrentRelease() (version string, url string, err error) {
	resp, err := http.Get("https://api.github.com/repos/evansiroky/timezone-boundary-builder/releases")
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
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

func writeData(content string, dir string) error {
	err := os.Chdir(dir)
	if err != nil {
		log.Printf("Error: could not switch to previous dir: %v", err)
		return err
	}

	outfile, err := os.Create("data/tzshapefile.go")
	if err != nil {
		log.Printf("Error: could not create tzshapefile.go: %v", err)
		return err
	}
	defer outfile.Close()

	_, err = outfile.WriteString(content)
	if err != nil {
		log.Printf("Error: could not write content: %v", err)
		return err
	}
	return nil
}

func writeVersion(release string, dir string) error {
	content := fmt.Sprintf(versionTemplate, release)
	err := os.Chdir(dir)
	if err != nil {
		log.Printf("Error: could not switch to previous dir: %v", err)
		return err
	}

	outfile, err := os.Create("version.go")
	if err != nil {
		log.Printf("Error: could not create version.go: %v", err)
		return err
	}
	defer outfile.Close()

	_, err = outfile.WriteString(content)
	if err != nil {
		log.Printf("Error: could not write content: %v", err)
		return err
	}
	return nil
}

func main() {
	mapshaperPath, err := exec.LookPath("mapshaper")
	if errors.Is(err, exec.ErrDot) {
		err = nil
	}
	if err != nil {
		log.Fatalln("Error: mapshaper executable not found in $PATH")
	}
	cwd, err := os.Getwd()
	if err != nil {
		log.Println(err)
	}
	mapshaperPath = path.Join(cwd, mapshaperPath)

	release := flag.String("release", defaultRelease, "timezone boundary builder release version")
	flag.Parse()

	var releaseURL string
	if *release == defaultRelease {
		*release, releaseURL, err = getMostCurrentRelease()
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		releaseURL = fmt.Sprintf(dlURL, *release)
	}
	resp, err := http.Get(releaseURL)
	if err != nil {
		log.Fatalf("Error: could not download tz shapefile: %v\n", err)
	}
	defer resp.Body.Close()

	buffer := bytes.NewBuffer([]byte{})
	_, err = io.Copy(buffer, resp.Body)
	if err != nil {
		log.Printf("Download failed: %v\n", err)
		return
	}

	bufferReader := bytes.NewReader(buffer.Bytes())
	zipReader, err := zip.NewReader(bufferReader, resp.ContentLength)
	if err != nil {
		log.Printf("Could not access zipfile: %v\n", err)
		return
	}
	if len(zipReader.File) == 0 {
		log.Println("Error: release zip file have no files!")
		return
	} else if zipReader.File[0].Name != "combined.json" {
		log.Println("Error: first file in zip file is not combined.json")
		return
	}

	geojsonData, err := zipReader.File[0].Open()
	if err != nil {
		log.Printf("Error: could not read from zip file: %v\n", err)
		return
	}

	currDir, err := os.Getwd()
	if err != nil {
		log.Printf("Error: could not get current dir: %v\n", err)
		return
	}

	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		log.Printf("Error: could not create tmp dir: %v\n", err)
		return
	}

	err = os.Chdir(tmpDir)
	if err != nil {
		log.Printf("Error: could not switch to tmp dir: %v\n", err)
		return
	}

	geojsonFile, err := os.Create("./combined.json")
	if err != nil {
		log.Printf("Error: could not create combinedJSON file: %v\n", err)
		return
	}

	_, err = io.Copy(geojsonFile, geojsonData)
	if err != nil {
		geojsonFile.Close()
		log.Printf("Error: could not copy from zip to combined.json: %v\n", err)
		return
	}
	geojsonFile.Close()

	fmt.Println("*** RUNNING MAPSHAPER ***")
	mapshaper := exec.Command(mapshaperPath, "-i", "combined.json", "-simplify", "visvalingam", "20%", "-o", "reduced.json")
	if errors.Is(mapshaper.Err, exec.ErrDot) {
		mapshaper.Err = nil
		fmt.Println("asdf")
	}
	mapshaper.Stdout = os.Stdout
	mapshaper.Stderr = os.Stderr
	err = mapshaper.Run()
	if err != nil {
		log.Printf("Error: could not run mapshaper: %v\n", err)
		return
	}
	fmt.Println("*** MAPSHAPER FINISHED ***")

	fmt.Println("*** GENERATING GO CODE ***")
	reducedFile, err := os.Open("reduced.json")
	if err != nil {
		log.Printf("Error: could not open file: %v\n", err)
		return
	}
	defer reducedFile.Close()

	buffer = bytes.NewBuffer([]byte{})
	gzipper, err := gzip.NewWriterLevel(buffer, gzip.BestCompression)
	if err != nil {
		log.Printf("Error: could not create gzip writer: %v\n", err)
		return
	}

	_, err = io.Copy(gzipper, reducedFile)
	if err != nil {
		log.Printf("Error: could not copy data: %v\n", err)
		return
	}
	if err := gzipper.Close(); err != nil {
		log.Printf("Error: could not flush/close gzip: %v\n", err)
		return
	}

	hexEncoded := bytes.NewBuffer([]byte{})
	for _, v := range buffer.Bytes() {
		hexEncoded.WriteString("\\x" + fmt.Sprintf("%02X", v))
	}
	content := fmt.Sprintf(dataTemplate, hexEncoded)

	err = writeData(content, currDir)
	if err != nil {
		return
	}

	err = writeVersion(*release, currDir)
	if err != nil {
		return
	}

	os.RemoveAll(tmpDir)
	fmt.Println("*** ALL DONE, YAY ***")
}
