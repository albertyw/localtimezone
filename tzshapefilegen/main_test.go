package main

import (
	"testing"

	"github.com/albertyw/localtimezone/v2/data"
)

func TestGetMostCurrentRelease(t *testing.T) {
	version, url, err := getMostCurrentRelease()
	if err != nil {
		t.Errorf("cannot get most current timezone boundary")
	}
	if url == "" {
		t.Errorf("cannot get most current timezone url")
	}
	if version != data.TZBoundaryVersion {
		t.Errorf("timezone boundary is out of date")
	}
}
