#!/bin/bash -e

# Setup JWT/OIDC authentication for OAuth-based access to AppRole credentials
# This configures the bridge between OAuth authentication and AppRole credential retrieval

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
JWT_POLICY_DIR="$SCRIPT_DIR/../jwt_policy_files"

echo "Setting up JWT authentication bridge policies..."

# Load JWT bridge policies from files
vault policy write trcshhivez-approle-read "$JWT_POLICY_DIR/trcshhivez-approle-read.hcl"
vault policy write trcshunrestricted-approle-read "$JWT_POLICY_DIR/trcshunrestricted-approle-read.hcl"

echo "JWT bridge policies created successfully"
echo ""
echo "Next steps:"
echo "1. Enable JWT auth method: vault auth enable jwt"
echo "2. Configure OIDC settings: vault write auth/jwt/config ..."
echo "3. Create JWT roles with these policies"
echo "   - Use policies='trcshhivez-approle-read' for standard access"
echo "   - Use policies='trcshunrestricted-approle-read' for unrestricted access"
