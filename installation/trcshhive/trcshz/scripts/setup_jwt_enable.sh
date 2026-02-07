#!/bin/bash -e
vault auth enable jwt

# Configure JWT/OIDC auth method (only once for both roles)
vault write auth/jwt/config \
    oidc_discovery_url="{{ .oauth_discovery_url }}" \
    oidc_client_id="{{ .oauth_client_id }}" \
    oidc_client_secret="{{ .oauth_client_secret }}" \
    default_role="trcshhivez"

# Create JWT role for standard read-only access (hardcoded - matches code expectations)
vault write auth/jwt/role/trcshhivez \
    bound_audiences="{{ .oauth_client_id }}" \
    bound_claims=@allowed_trcsh_users.json \
    user_claim="email" \
    role_type="jwt" \
    policies="trcshhivez-approle-manager" \
    ttl=5m

# Create JWT role for unrestricted write access (hardcoded - matches code expectations)
vault write auth/jwt/role/trcshunrestricted \
    bound_audiences="{{ .oauth_client_id }}" \
    bound_claims=@allowed_trcsh_unrestricted_users.json \
    user_claim="email" \
    role_type="jwt" \
    policies="trcshunrestricted-approle-manager" \
    ttl=5m
