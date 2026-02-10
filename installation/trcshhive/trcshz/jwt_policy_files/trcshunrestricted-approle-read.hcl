# Policy to allow JWT-authenticated users to retrieve trcshunrestricted AppRole credentials
# This policy should be attached to the JWT role used for OAuth user authentication with unrestricted write access

path "auth/approle/role/trcshunrestricted/role-id" {
  capabilities = ["read"]
}

path "auth/approle/role/trcshunrestricted/secret-id" {
  capabilities = ["update"]
}
