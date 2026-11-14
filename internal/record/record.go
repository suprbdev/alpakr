// Package record defines the ordered object that handlers produce.
package record

import (
	"bytes"
	"encoding/json"
)

// Object is an output object whose keys keep the order they were defined
// in. Keys may be shared between objects built from the same field list and
// must not be modified.
type Object struct {
	Keys   []string
	Values []interface{}
}

// Get returns the value for key and whether it is present.
func (o *Object) Get(key string) (interface{}, bool) {
	for i, k := range o.Keys {
		if k == key {
			return o.Values[i], true
		}
	}
	return nil, false
}

// MarshalJSON encodes o with its keys in order and without HTML escaping.
func (o *Object) MarshalJSON() ([]byte, error) {
	return AppendJSON(nil, o)
}

// AppendJSON appends the compact JSON encoding of v to dst. Objects (also
// inside arrays) are written in key order; other values are encoded by
// encoding/json without HTML escaping.
func AppendJSON(dst []byte, v interface{}) ([]byte, error) {
	buf := bytes.NewBuffer(dst)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := appendValue(buf, enc, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func appendValue(buf *bytes.Buffer, enc *json.Encoder, v interface{}) error {
	switch x := v.(type) {
	case *Object:
		buf.WriteByte('{')
		for i, k := range x.Keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := appendScalar(buf, enc, k); err != nil {
				return err
			}
			buf.WriteByte(':')
			if err := appendValue(buf, enc, x.Values[i]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
		return nil
	case []interface{}:
		buf.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := appendValue(buf, enc, e); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
		return nil
	}
	return appendScalar(buf, enc, v)
}

// appendScalar encodes v with enc and drops the newline Encode appends.
func appendScalar(buf *bytes.Buffer, enc *json.Encoder, v interface{}) error {
	if err := enc.Encode(v); err != nil {
		return err
	}
	buf.Truncate(buf.Len() - 1)
	return nil
}
