package main

import (
	"bytes"

	"gopkg.in/yaml.v3"
)

func MarshalYAMLWithIndent(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	err := enc.Encode(v)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
