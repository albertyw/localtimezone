package localtimezone

import (
	json "github.com/goccy/go-json"
)

type unmarshaler struct{}

func (u unmarshaler) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
