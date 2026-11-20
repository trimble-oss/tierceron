// Package lockedjson extracts a string from a bounded Vault response without
// first decoding it into a Go heap string.
package lockedjson

import (
	"bytes"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/trimble-oss/tierceron-core/v2/buildopts/memprotectopts"
)

const maxResponse = 1 << 20

// ReadString reads a nested string into locked storage. The caller owns the
// result and must release it with LockedFree or process-wide cleanup.
func ReadString(r io.Reader, path ...string) ([]byte, error) {
	// Prefix JSON whitespace so even an empty response has a freeable slice.
	body, err := memprotectopts.LockedReadAll(io.MultiReader(strings.NewReader(" "), io.LimitReader(r, maxResponse+1)))
	if err != nil {
		return nil, fmt.Errorf("read locked response: %w", err)
	}
	var value []byte
	if len(body) > maxResponse+1 {
		err = errors.New("locked JSON response exceeds 1 MiB")
	} else {
		value, err = decodeString(body, path)
	}
	if err = errors.Join(err, memprotectopts.LockedFree(body)); err != nil {
		return nil, errors.Join(err, memprotectopts.LockedFree(value))
	}
	return value, nil
}

func decodeString(body []byte, path []string) ([]byte, error) {
	raw := jsontext.Value(body)
	for _, name := range path {
		var err error
		raw, err = member(raw, name)
		if err != nil {
			// Decoder errors can quote response data.
			return nil, err
		}
	}
	if raw.Kind() != '"' {
		return nil, errors.New("missing or invalid locked JSON string")
	}
	buf, err := memprotectopts.LockedAlloc(len(raw))
	if err != nil {
		return nil, fmt.Errorf("allocate locked JSON string: %w", err)
	}
	value, err := jsontext.AppendUnquote(buf[:0], raw)
	if err != nil || len(value) == 0 {
		return nil, errors.Join(errors.New("invalid or empty locked JSON string"), memprotectopts.LockedFree(buf))
	}
	return value, nil
}

func member(input jsontext.Value, name string) (jsontext.Value, error) {
	// The decoder scans the existing bytes.Buffer storage directly.
	d := jsontext.NewDecoder(bytes.NewBuffer(input))
	if d.PeekKind() != '{' {
		return nil, errors.New("expected locked JSON object")
	}
	if _, err := d.ReadToken(); err != nil {
		return nil, errors.New("invalid locked JSON object")
	}
	var found jsontext.Value
	for d.PeekKind() == '"' {
		key, err := d.ReadToken()
		if err != nil {
			return nil, errors.New("invalid locked JSON field name")
		}
		match := key.String() == name
		value, err := d.ReadValue()
		if err != nil {
			return nil, errors.New("invalid locked JSON field value")
		}
		if match {
			// Keep offsets into the input, not a decoder-owned view.
			end := d.InputOffset()
			found = input[end-int64(len(value)) : end]
		}
	}
	if d.PeekKind() != '}' {
		return nil, errors.New("invalid locked JSON object")
	}
	if _, err := d.ReadToken(); err != nil {
		return nil, errors.New("invalid locked JSON object")
	}
	if _, err := d.ReadValue(); err != io.EOF {
		return nil, errors.New("unexpected data after locked JSON object")
	}
	return found, nil
}
