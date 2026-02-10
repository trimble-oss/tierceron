# Policy to allow JWT-authenticated users to retrieve trcshhivez AppRole credentials
# This policy should be attached to the JWT role used for OAuth user authentication

path "auth/approle/role/trcshhivez/role-id" {
  capabilities = ["read"]
}

path "auth/approle/role/trcshhivez/secret-id" {
  capabilities = ["update"]
}
