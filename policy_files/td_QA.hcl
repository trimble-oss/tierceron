path "templates/*" {
  capabilities = ["create", "update"]
}

path "values/data/QA/*" {
  capabilities = ["create", "update"]
}

path "value-metrics/data/QA/*" {
  capabilities = ["create", "update"]
}