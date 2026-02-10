# OAuth/JWT Configuration for trcsh

This configuration enables browser-based OAuth authentication for trcsh using an oauth identity provider and Vault JWT auth.

## Configuration Files

- **config.yml.tmpl** - Go template file used for deployment
- **config.yml.example** - Example with default values

## Required Variables

### Vault Connection
- `vault_addr` - Vault server address (e.g., `https://vault.example.com`)
- `agent_env` - Environment name (dev, staging, prod)
- `region` - Cloud region
- `deployments` - Deployment type

### OAuth/OIDC Settings
These are obtained from your OAuth provider registration:

- `oauth_discovery_url` - OIDC discovery endpoint (used by client for OAuth flow)
  - Your ID provider: `https://<provider>/.well-known/openid-configuration`

- `oauth_jwks_url` - JWKS endpoint (used by Vault for JWT validation)
  - Your ID provider: `https://<provider>/.well-known/jwks.json`
  
- `oauth_client_id` - Your OAuth application client ID
  - Register at Identity Provider
  
- `oauth_client_secret` - Client secret (optional for public clients)
  - Leave empty if using PKCE without client secret
  
- `oauth_callback_port` - Local port for OAuth callback server
  - Default: `8080`
  - Redirect URL is automatically set to `http://localhost:<port>/callback`
  - Must register this URL with OAuth provider (e.g., `http://localhost:8080/callback`)

### Vault Role Configuration
Must match your Vault setup:

- `vault_role` - Vault role name for read-only access (used for both JWT auth and AppRole)
  - Default: `trcshhivez`
  - The JWT role and AppRole have the same name by design
  - Used for reading configuration tokens
  - Created during Vault setup (see OAUTH_VAULT_INTEGRATION.md)

- `vault_unrestricted_role` - Vault role name for write access (optional)
  - Default: `trcshunrestricted`
  - Used for modifying configuration tokens
  - Requires separate OAuth authentication
  - More restrictive user whitelist than read-only role

## Vault Roles

### trcshhivez (Read-Only Access)
- **Purpose**: Default authentication for trcsh with read-only access
- **Token Pattern**: `trcsh_agent_{env}` (dev, QA, RQA, performance, servicepack, staging, prod)
- **Authentication**: Automatic on trcsh startup
- **Use Cases**: Reading secrets, viewing configurations, normal operations
- **User Access**: Broad whitelist for developers and operators

### trcshunrestricted (Write Access)
- **Purpose**: Write access to configuration tokens
- **Token Pattern**: `config_{env}_unrestricted` (dev, QA, RQA, performance, servicepack, staging)
- **Authentication**: On-demand via `utils.GetUnrestrictedAccess(driverConfig)`
- **Use Cases**: Modifying configurations, updating secrets, administrative changes
- **User Access**: Highly restricted whitelist for administrators only

**To obtain write access in trcsh:**
```go
import "github.com/trimble-oss/tierceron/pkg/utils"

// When you need unrestricted write access in a command:
err := utils.GetUnrestrictedAccess(driverConfig)
if err != nil {
    return fmt.Errorf("failed to get unrestricted access: %w", err)
}
// Now you have write access credentials loaded
```

## Setup Steps

1. **Register OAuth Application**
   - Go to Identity Provider Portal
   - Create new application
   - Set redirect URI to `http://localhost:8080/callback`
   - Save the client_id (and client_secret if provided)

2. **Configure Vault** (Admin only)
   - Enable JWT auth method
   - Configure OIDC provider
   - Create JWT role `trcshhivez` with read user whitelist
   - Create AppRole `trcshhivez` with read-only policies
   - Create policy `trcshhivez-approle-read` for AppRole credential retrieval
   - (Optional) Create JWT role `trcshunrestricted` with admin user whitelist
   - (Optional) Create AppRole `trcshunrestricted` with write-access policies
   - (Optional) Create policy `trcshunrestricted-approle-read` for AppRole credential retrieval

   See: `/home/jrieke/workspace/Github/trimble-oss/tierceron/pkg/vaulthelper/system/OAUTH_VAULT_INTEGRATION.md`

3. **Set Configuration Values**
   - Copy `config.yml.example` to your values file
   - Update `oauth_client_id` with your OAuth client ID
   - Update `oauth_client_secret` if using confidential client
   - Update `vault_addr` with your Vault server address
   - Adjust other values as needed

4. **Deploy Template**
   - Process template with your values
   - Deploy to target environment
   - Users can now run `trcsh login` to authenticate
### Initial Startup (Read-Only Access)
1. User runs trcsh
2. System checks `~/.tierceron/config.yml` for cached credentials
3. If no valid credentials:
   - Browser opens to Identity provider login page
   - User authenticates with their Identity credentials
   - Identity provider redirects back with authorization code
   - trcsh exchanges code for ID token using PKCE
4. trcsh presents ID token to Vault JWT auth for `trcshhivez` role
5. Vault validates JWT and checks email against allowed list
6. Vault issues token with policies for reading AppRole credentials
7. trcsh retrieves AppRole credentials (role-id + secret-id) for `trcshhivez`
8. AppRole credentials cached locally for future sessions
9. trcsh uses AppRole to access read-only configuration tokens

### On-Demand Write Access
When a trcsh command needs write access:
1. Command calls `utils.GetUnrestrictedAccess(driverConfig)`
2. System checks `~/.tierceron/config.yml` for cached unrestricted credentials
3. If no valid credentials:cached in `~/.tierceron/config.yml` (0600 permissions)
- **Separate from Kubernetes** - trcshhivez/trcshunrestricted AppRoles are separate from hivekernel
- **Role-based access**:
  - `trcshhivez`: Broad user whitelist for read operations
  - `trcshunrestricted`: Highly restricted admin whitelist for write operations
- **Separate credential caching** - Read and write credentials are cached separately
   - User authenticates with their Identity provider credentials
   - trcsh exchanges for new ID token
4. trcsh presents ID token to Vault JWT auth for `trcshunrestricted` role
5. Vault validates JWT and checks email against admin whitelist
6. trcsh retrieves AppRole credentials for `trcshunrestricted`
7. Unrestricted credentials cached separately in config file
  - `trcshhivez`: Broad developer/operator list
  - `trcshunrestricted`: Restricted administrator list
- JWT role's `bound_audiences` must match `oauth_client_id`
- Vault must be able to reach OIDC discovery URL for JWKS
- "User not authorized for this role" error means email not in whitelist for that specific role
9. trcsh uses Vault token to retrieve AppRole credentials
10. AppRole credentials stored locally for future sessions

## Security Notes

- **Client credentials alone DO NOT grant access** - user must authenticate
- **Email validation** - Vault checks JWT email claim against allowed list
- **JWT signature verification** - Cannot be forged by client
- **Credential storage** - AppRole credentials should be encrypted at rest
- **Separate from Kubernetes** - trcshhivez AppRole is separate from hivekernel

## Troubleshooting

### Configuration Issues
- Verify `oauth_client_id` matches what's registered with OAuth provider
- Ensure redirect URL `http://localhost:<oauth_callback_port>/callback` is registered with OAuth provider
- Check `vault_addr` is accessible from client

### Authentication Failures
- User email must be in JWT role's `bound_claims` list
- JWT role's `bound_audiences` must match `oauth_client_id`
- Vault must be able to reach `oauth_jwks_url` for public key validation

### Network Issues
- Port 8080 must be available for OAuth callback
- Firewall must allow s: 
  - Read-only: `/home/jrieke/workspace/Github/trimble-oss/VaultConfig.Bootstrap/vault_namespaces/vault/approle_files/trcshhivez.yml`
  - Write access: `/home/jrieke/workspace/Github/trimble-oss/VaultConfig.Bootstrap/vault_namespaces/vault/approle_files/trcshunrestricted.yml`
- Vault Policies:
  - `/home/jrieke/workspace/Github/trimble-oss/VaultConfig.Bootstrap/vault_namespaces/vault/policy_files/trcshhivez-approle-read.hcl`
  - `/home/jrieke/workspace/Github/trimble-oss/VaultConfig.Bootstrap/vault_namespaces/vault/policy_files/trcshunrestricted-approle-read.hc
- Vault must be network reachable

## References

- OAuth Implementation: `/home/jrieke/workspace/Github/trimble-oss/tierceron/pkg/oauth/README.md`
- Vault Integration: `/home/jrieke/workspace/Github/trimble-oss/tierceron/pkg/vaulthelper/system/OAUTH_VAULT_INTEGRATION.md`
- Vault AppRole Config: `/home/jrieke/workspace/Github/trimble-oss/VaultConfig.Bootstrap/vault_namespaces/vault/approle_files/trcshhivez.yml`
