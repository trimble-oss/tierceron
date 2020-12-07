path "templates/*" {
  capabilities = ["read", "create", "update", "list"]
}

path "values/data/performance/*" {
  capabilities = ["create", "update"]
}

path "value-metrics/data/performance/*" {
  capabilities = ["create", "update"]
}
