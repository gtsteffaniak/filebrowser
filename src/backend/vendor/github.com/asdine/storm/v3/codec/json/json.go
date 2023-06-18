// Package json contains a codec to encode and decode entities in JSON format
package json

import (
	"encoding/json"
)

const name = "json"

// Codec that encodes to and decodes from JSON.
var Codec = new(jsonCodec)

type jsonCodec int

func (j jsonCodec) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (j jsonCodec) Unmarshal(b []byte, v interface{}) error {
	return json.Unmarshal(b, v)
}

func (j jsonCodec) Name() string {
	return name
}
