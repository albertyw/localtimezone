package main

import (
	"github.com/goccy/go-json"
)

type marshaler struct{}

func (u marshaler) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (u marshaler) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
