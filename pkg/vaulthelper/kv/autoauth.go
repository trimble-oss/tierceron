package kv

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
	"unsafe"

	"github.com/trimble-oss/tierceron-core/v2/buildopts/memonly"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/internal/lockedjson"
)

// ErrAutoAuthTokenDenied allows AutoAuth to try its protected token fallback.
var ErrAutoAuthTokenDenied = errors.New("Vault denied the AutoAuth token read")

// ReadAutoAuthToken reads the named token from the current environment's token
// record. In memonly builds it decodes only that token into a locked allocation
// that remains valid until process-wide memory cleanup.
func (m *Modifier) ReadAutoAuthToken(key string) (string, error) {
	if !memonly.IsMemonly() {
		return m.ReadValue("super-secrets/tokens", key)
	}
	if m.client.OutputCurlString() {
		return "", errors.New("cannot print a memonly AutoAuth token request")
	}
	for range 3 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		token, err := m.readLockedAutoAuthToken(ctx, key)
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			return token, err
		}
	}
	m.Stale = true
	m.httpClient.CloseIdleConnections()
	return "", fmt.Errorf("AutoAuth token read timed out after 3 attempts: %w", context.DeadlineExceeded)
}

func (m *Modifier) readLockedAutoAuthToken(ctx context.Context, key string) (string, error) {
	response, err := m.logical.ReadRawWithContext(ctx, "super-secrets/data/"+m.Env+"/tokens")
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	if response != nil && response.StatusCode == http.StatusForbidden {
		return "", ErrAutoAuthTokenDenied
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if response != nil && response.StatusCode >= 300 {
		return "", fmt.Errorf("Vault AutoAuth token read returned HTTP %d", response.StatusCode)
	}
	if err != nil {
		return "", fmt.Errorf("read Vault AutoAuth token record: %w", err)
	}
	if response == nil || response.Body == nil {
		return "", errors.New("missing Vault AutoAuth token response")
	}
	token, err := lockedjson.ReadString(response.Body, "data", "data", key)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("read Vault AutoAuth token: %w", err)
	}
	return unsafe.String(unsafe.SliceData(token), len(token)), nil
}
