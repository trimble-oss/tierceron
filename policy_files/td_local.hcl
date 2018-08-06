path "templates/*" {
  capabilities = ["create", "update"]
}

path "values/data/local/*" {
  capabilities = ["create", "update"]
}

path "value-metrics/data/local/*" {
  capabilities = ["create", "update"]
}