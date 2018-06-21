path "templates/*" {
  capabilities = ["read", "list"]
}

path "values/data/QA/*" {
  capabilities = ["read", "list"]
}

path "super-secrets/data/QA/*" {
  capabilities = ["read", "list"]
}

path "value-metrics/data/*" {
  capabilities = ["read", "list", "create", "update"]
}
