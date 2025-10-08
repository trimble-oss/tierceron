path "templates/*" {
  capabilities = ["read", "list", "create", "update"]
}

path "values/metadata/local/*" {
  capabilities = ["read", "list", "create", "update", "delete"]
}

path "values/data/local/*" {
  capabilities = ["read", "list", "create", "update", "delete"]
}

path "super-secrets/metadata/local/*" {
  capabilities = ["read", "list", "create", "update", "delete"]
}

path "super-secrets/data/local/*" {
  capabilities = ["read", "list", "create", "update", "delete"]
}

path "value-metrics/metadata/local/*" {
  capabilities = ["read", "list", "create", "update", "delete"]
}

path "value-metrics/data/local/*" {
  capabilities = ["read", "list", "create", "update", "delete"]
}

path "verification/data/local*" { 
  capabilities = ["read", "list", "create", "update", "delete"]
}

path  "apiLogins/data/local*" {
  capabilities = ["read", "list", "create", "update", "delete"]
}
