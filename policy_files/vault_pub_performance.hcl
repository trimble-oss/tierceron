path "templates/*" {
  capabilities = ["create", "update"]
}

path "values/data/performance/*" {
  capabilities = ["create", "update"]
}

path "value-metrics/data/performance/*" {
  capabilities = ["create", "update"]
}
