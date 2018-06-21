path "templates/*" {
  capabilities = ["create", "update"]
}

path "values/data/dev*" {
  capabilities = ["create", "update"]
}

path "value-metrics/data/dev/*" {
  capabilities = ["create", "update"]
}