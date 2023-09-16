package system

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/vault/api"
)

// NewRoleOptions is used to create a new approle
type NewRoleOptions struct {
	BindSecretID         bool     `json:"bind_secret_id,omitempty"`
	SecretIDBoundCIDRs   []string `json:"secret_id_bound_cidrs,omitempty"`
	TokenBoundCIDRs      []string `json:"token_bound_cidrs,omitempty"`
	Policies             []string `json:"policies"`
	SecretIDTTL          string   `json:"secret_id_num_uses,omitempty"`
	TokenNumUses         int      `json:"token_num_uses,omitempty"`
	TokenTTL             string   `json:"token_ttl,omitempty"`
	TokenMaxTTL          string   `json:"token_max_ttl,omitempty"`
	Period               string   `json:"period,omitempty"`
	EnableLocalSecretIDs string   `json:"enable_local_secret_ids,omitempty"`
}

// YamlNewTokenRoleOptions is used to create a new approle
type YamlNewTokenRoleOptions struct {
	RoleName        string   `yaml:"role_name,omitempty"`
	TokenBoundCIDRs []string `yaml:"token_bound_cidrs,omitempty"`
}

// NewTokenRoleOptions is used to create a new approle
type NewTokenRoleOptions struct {
	RoleName             string   `json:"role_name,omitempty"`
	AllowedPolicies      []string `json:"allowed_policies,omitempty"`
	DisallowedPolicies   []string `json:"disallowed_policies,omitempty"`
	Orphan               bool     `json:"orphan,omitempty"`
	Renewable            bool     `json:"renewable,omitempty"`
	PathSuffix           string   `json:"path_suffix,omitempty"`
	AllowedEntityAliases []string `json:"allowed_entity_aliases,omitempty"`
	TokenBoundCIDRs      []string `json:"token_bound_cidrs,omitempty"`
	TokenExplicitMaxTTL  int      `json:"token_explicit_max_ttl,omitempty"`
	TokenNoDefaultPolicy bool     `json:"token_no_default_policy,omitempty"`
	TokenNumUses         int      `json:"token_num_uses,omitempty"`
	TokenPeriod          int      `json:"token_period,omitempty"`
	TokenType            string   `json:"token_type,omitempty"`
}

// EnableAppRole enables the app role auth method and returns any errors
func (v *Vault) EnableAppRole() error {
	sys := v.client.Sys()
	err := sys.EnableAuthWithOptions("approle", &api.EnableAuthOptions{
		Type:        "approle",
		Description: "Auth endpoint for vault config",
		Config: api.AuthConfigInput{
			DefaultLeaseTTL: "10m",
			MaxLeaseTTL:     "15m",
		},
	})
	return err
}

// CreateNewRole creates a new role with given options
func (v *Vault) CreateNewRole(roleName string, options *NewRoleOptions) error {
	r := v.client.NewRequest("POST", fmt.Sprintf("/v1/auth/approle/role/%s", roleName))
	if err := r.SetJSONBody(options); err != nil {
		return err
	}

	response, err := v.client.RawRequest(r)

	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	return err
}

// DeleteRole deletes role with given role name
func (v *Vault) DeleteRole(roleName string) (*api.Response, error) {
	r := v.client.NewRequest("DELETE", fmt.Sprintf("/v1/auth/approle/role/%s", roleName))

	response, err := v.client.RawRequest(r)
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	return response, err
}

// CreateNewTokenCidrRole creates a new token cidr only role with given cidr options.
func (v *Vault) CreateNewTokenCidrRole(options *YamlNewTokenRoleOptions) error {
	rolePath := fmt.Sprintf("/v1/auth/token/roles/%s", options.RoleName)
	r := v.client.NewRequest("POST", rolePath)
	limitedTokenRole := NewTokenRoleOptions{}
	limitedTokenRole.TokenBoundCIDRs = options.TokenBoundCIDRs

	if err := r.SetJSONBody(options); err != nil {
		return err
	}

	response, err := v.client.RawRequest(r)

	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	return err
}

// GetRoleID checks for the given role name and returns the coresponding id if it exists
func (v *Vault) GetRoleID(roleName string) (string, string, error) {
	r := v.client.NewRequest("GET", fmt.Sprintf("/v1/auth/approle/role/%s/role-id", roleName))
	response, err := v.client.RawRequest(r)

	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}

	if err != nil {
		return "", "", err
	}

	var jsonData map[string]interface{}
	if err = response.DecodeJSON(&jsonData); err != nil {
		return "", "", err
	}

	if raw, ok := jsonData["data"].(map[string]interface{}); ok {
		if roleID, ok := raw["role_id"].(string); ok {
			return roleID, string(jsonData["lease_duration"].(json.Number)), nil
		}
		return "", "", fmt.Errorf("Error parsing response for key 'data.id'")
	}

	return "", "", fmt.Errorf("Error parsing resonse for key 'data'")
}

// GetSecretID checks the vault for the secret ID corresponding to the role name
func (v *Vault) GetSecretID(roleName string) (string, error) {
	r := v.client.NewRequest("POST", fmt.Sprintf("/v1/auth/approle/role/%s/secret-id", roleName))
	response, err := v.client.RawRequest(r)

	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}

	if err != nil {
		return "", err
	}

	var jsonData map[string]interface{}
	if err = response.DecodeJSON(&jsonData); err != nil {
		return "", err
	}

	if raw, ok := jsonData["data"].(map[string]interface{}); ok {
		if secretID, ok := raw["secret_id"].(string); ok {
			return secretID, nil
		}
		return "", fmt.Errorf("Error parsing response for key 'data.secret_id'")
	}

	return "", fmt.Errorf("Error parsing resonse for key 'data'")
}

// GetListApproles lists available approles
func (v *Vault) GetListApproles() (string, error) {
	r := v.client.NewRequest("LIST", "/v1/auth/approle/role")
	response, err := v.client.RawRequest(r)

	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}

	if err != nil {
		return "", err
	}

	var jsonData map[string]interface{}
	if err = response.DecodeJSON(&jsonData); err != nil {
		return "", err
	}

	return "", fmt.Errorf("Error parsing resonse for key 'data'")
}

// AppRoleLogin tries logging into the vault using app role and returns a client token on success
func (v *Vault) AppRoleLogin(roleID string, secretID string) (string, error) {
	r := v.client.NewRequest("POST", "/v1/auth/approle/login")

	payload := map[string]interface{}{
		"role_id":   roleID,
		"secret_id": secretID,
	}

	if err := r.SetJSONBody(payload); err != nil {
		return "", err
	}

	response, err := v.client.RawRequest(r)
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}

	if err != nil {
		return "", err
	}

	var jsonData map[string]interface{}

	if err = response.DecodeJSON(&jsonData); err != nil {
		return "", err
	}

	if authData, ok := jsonData["auth"].(map[string]interface{}); ok {
		if token, ok := authData["client_token"].(string); ok {
			return token, nil
		}
		return "", fmt.Errorf("Error parsing response for key 'auth.client_token'")
	}

	return "", fmt.Errorf("Error parsing response for key 'auth'")
}
