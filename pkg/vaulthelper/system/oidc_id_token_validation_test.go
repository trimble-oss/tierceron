package system

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/trimble-oss/tierceron-core/v2/identity"
)

func TestValidateOIDCIDTokenUsesVaultRoleAndRevokesTemporaryIdentity(t *testing.T) {
	for _, denied := range []bool{false, true} {
		t.Run(fmt.Sprint(denied), func(t *testing.T) {
			var logins, revocations atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v1/auth/jwt/login":
					logins.Add(1)
					var request struct {
						JWT  string `json:"jwt"`
						Role string `json:"role"`
					}
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.JWT == "" || request.Role != "trcshunrestricted" {
						t.Error("invalid Vault authentication request")
					}
					if denied {
						w.WriteHeader(http.StatusForbidden)
						fmt.Fprint(w, `{"errors":["denied"]}`)
						return
					}
					fmt.Fprint(w, `{"auth":{"client_token":"temporary-test-token"}}`)
				case "/v1/auth/token/revoke-self":
					revocations.Add(1)
					if r.Header.Get("X-Vault-Token") != "temporary-test-token" {
						t.Error("temporary identity was not used for revocation")
					}
					w.WriteHeader(http.StatusNoContent)
				default:
					t.Errorf("unexpected Vault endpoint %q", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			client, err := api.NewClient(&api.Config{Address: server.URL, HttpClient: server.Client()})
			if err != nil {
				t.Fatal(err)
			}
			client.SetToken("original-test-token")
			vault := &Vault{client: client}
			payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, time.Now().Add(time.Hour).Unix())))
			token := "e30." + payload + ".signature"
			err = vault.ValidateOIDCIDToken(context.Background(), token)
			expectedRevocations := int32(1)
			if denied {
				expectedRevocations = 0
			}
			if (err != nil) != denied || logins.Load() != 1 || revocations.Load() != expectedRevocations || client.Token() != "original-test-token" {
				t.Fatalf("error=%v logins=%d revocations=%d", err, logins.Load(), revocations.Load())
			}
			payload = base64.RawURLEncoding.EncodeToString([]byte(`{"exp":1}`))
			if err := vault.ValidateOIDCIDToken(context.Background(), "e30."+payload+".signature"); !errors.Is(err, identity.ErrOIDCIDTokenExpired) || logins.Load() != 1 {
				t.Fatalf("expired identity reached Vault: error=%v logins=%d", err, logins.Load())
			}
		})
	}
}
