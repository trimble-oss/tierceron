package system

import (
	"bytes"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"unsafe"

	"github.com/trimble-oss/tierceron-core/v2/buildopts/memprotectopts"
)

const maxAppRoleResponse = 1 << 20

func (v *Vault) lockedAppRoleLogin(roleID, secretID string) (*string, error) {
	if v.client.OutputCurlString() {
		return nil, errors.New("cannot print a memonly AppRole login request")
	}
	for _, value := range []*string{&roleID, &secretID} {
		if err := protectMemory(nil, value); err != nil {
			return nil, err
		}
	}

	body, allocation, err := lockedAppRoleBody(roleID, secretID)
	if err != nil {
		return nil, err
	}
	r := v.client.NewRequest("POST", "/v1/auth/approle/login")
	// BodyBytes would cause Vault to regenerate the JSON body on redirects.
	reader := &lockedRequestReader{reader: bytes.NewReader(body)}
	r.Body = reader
	r.Headers.Set("Content-Type", "application/json")
	response, err := v.client.RawRequest(r)
	reader.mu.Lock()
	reader.reader = nil
	freeErr := memprotectopts.LockedFree(allocation)
	reader.mu.Unlock()
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	if err != nil || freeErr != nil {
		return nil, errors.Join(err, freeErr)
	}
	if response == nil || response.Body == nil {
		return nil, errors.New("missing AppRole response body")
	}
	return readLockedAppRoleToken(response.Body)
}

// The transport may still be reading when a server sends an early response.
// Invalidation waits for any in-flight read before the backing memory is freed.
type lockedRequestReader struct {
	mu     sync.Mutex
	reader *bytes.Reader
}

func (r *lockedRequestReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.reader == nil {
		return 0, io.ErrClosedPipe
	}
	return r.reader.Read(p)
}

func (r *lockedRequestReader) Seek(offset int64, whence int) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.reader == nil {
		return 0, io.ErrClosedPipe
	}
	return r.reader.Seek(offset, whence)
}

func (r *lockedRequestReader) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.reader == nil {
		return 0
	}
	return int(r.reader.Size())
}

func lockedAppRoleBody(roleID, secretID string) (body, allocation []byte, err error) {
	const prefix, middle, suffix = `{"role_id":`, `,"secret_id":`, `}`
	const overhead = len(prefix) + len(middle) + len(suffix) + 4
	const maxCredentialBytes = (int(^uint(0)>>1) - overhead) / 6
	if len(roleID) > maxCredentialBytes || len(secretID) > maxCredentialBytes-len(roleID) {
		return nil, nil, errors.New("AppRole request is too large")
	}
	allocation, err = memprotectopts.LockedAlloc(overhead + 6*(len(roleID)+len(secretID)))
	if err != nil {
		return nil, nil, fmt.Errorf("allocate locked AppRole request: %w", err)
	}
	body, err = appendAppRoleQuote(append(allocation[:0], prefix...), roleID)
	if err == nil {
		body, err = appendAppRoleQuote(append(body, middle...), secretID)
	}
	if err != nil {
		return nil, nil, errors.Join(err, memprotectopts.LockedFree(allocation))
	}
	return append(body, suffix...), allocation, nil
}

func appendAppRoleQuote(dst []byte, value string) ([]byte, error) {
	// Pass a view rather than a string-to-byte conversion.
	b, err := jsontext.AppendQuote(dst, unsafe.Slice(unsafe.StringData(value), len(value)))
	if err != nil {
		return nil, errors.New("invalid UTF-8 in AppRole credentials")
	}
	return b, nil
}

func readLockedAppRoleToken(r io.Reader) (*string, error) {
	// Prefix JSON whitespace so LockedReadAll always returns a freeable slice
	// even when the response is empty.
	body, err := memprotectopts.LockedReadAll(io.MultiReader(strings.NewReader(" "), io.LimitReader(r, maxAppRoleResponse+1)))
	if err != nil {
		return nil, fmt.Errorf("read locked AppRole response: %w", err)
	}
	var decoded []byte
	var allocation []byte
	if len(body) > maxAppRoleResponse+1 {
		err = errors.New("AppRole response exceeds 1 MiB")
	} else {
		decoded, allocation, err = decodeLockedAppRoleToken(body)
	}
	if err = errors.Join(err, memprotectopts.LockedFree(body)); err != nil {
		return nil, errors.Join(err, memprotectopts.LockedFree(allocation))
	}

	// The separate token allocation lives until process-wide memory cleanup.
	token := unsafe.String(unsafe.SliceData(decoded), len(decoded))
	if err := protectMemory(nil, &token); err != nil {
		return nil, errors.Join(err, memprotectopts.LockedFree(allocation))
	}
	return &token, nil
}

func decodeLockedAppRoleToken(body []byte) (decoded, allocation []byte, err error) {
	// jsontext parses directly from bytes.Buffer's existing storage.
	d := jsontext.NewDecoder(bytes.NewBuffer(body))
	auth, err := lockedJSONObjectMember(d, body, "auth")
	if err != nil || auth.Kind() != '{' {
		return nil, nil, errors.New("invalid AppRole response auth object")
	}
	if _, err := d.ReadValue(); err != io.EOF {
		return nil, nil, errors.New("unexpected data after AppRole response")
	}
	d = jsontext.NewDecoder(bytes.NewBuffer(auth))
	raw, err := lockedJSONObjectMember(d, auth, "client_token")
	if err != nil || raw.Kind() != '"' {
		return nil, nil, errors.New("invalid AppRole response client token")
	}
	allocation, err = memprotectopts.LockedAlloc(len(raw))
	if err != nil {
		return nil, nil, fmt.Errorf("allocate locked AppRole token: %w", err)
	}
	decoded, err = jsontext.AppendUnquote(allocation[:0], raw)
	if err != nil || len(decoded) == 0 {
		return nil, allocation, errors.New("invalid AppRole response client token")
	}
	return decoded, allocation, nil
}

// ReadValue's slice is valid only until the next decoder operation. Save its
// offsets and return a view into the original locked input instead.
func lockedJSONObjectMember(d *jsontext.Decoder, input []byte, name string) (jsontext.Value, error) {
	if d.PeekKind() != '{' {
		return nil, errors.New("expected JSON object")
	}
	if _, err := d.ReadToken(); err != nil {
		return nil, err
	}
	var found jsontext.Value
	for d.PeekKind() == '"' {
		key, err := d.ReadToken()
		if err != nil {
			return nil, err
		}
		if key.String() != name {
			if err := d.SkipValue(); err != nil {
				return nil, err
			}
			continue
		}
		value, err := d.ReadValue()
		if err != nil {
			return nil, err
		}
		end := d.InputOffset()
		found = input[end-int64(len(value)) : end]
	}
	if d.PeekKind() != '}' {
		return nil, errors.New("invalid JSON object")
	}
	_, err := d.ReadToken()
	return found, err
}
