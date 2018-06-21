path "templates/*" {
  capabilities = ["read", "list"]
}

path "values/data/dev/*" {
  capabilities = ["read", "list"]
}

path "super-secrets/data/dev/*" {
  capabilities = ["read", "list"]
}

path "value-metrics/data/*" {
  capabilities = ["read", "list", "create", "update"]
}
