path "templates/*" {
  capabilities = ["read", "create", "update", "list"]
}

path "values/data/dev/*" {
  capabilities = ["create", "update"]
}

path "value-metrics/data/dev/*" {
  capabilities = ["create", "update"]
}

path "values/data/dev/Restricted/*" {
  capabilities = ["deny"]
}
path "value-metrics/data/dev/Restricted/*" {
  capabilities = ["deny"]
}