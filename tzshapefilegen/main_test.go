package main

import (
	"testing"

	localtimezone "github.com/albertyw/localtimezone/v3"
)

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
