path "data/templates/*" {
  capabilities = ["read"]
}
path "data/values/*" {
    capabilities = ["read"]
}
path "data/super-secrets/*" {
  capabilities = ["read"]
}
path "data/value-metrics/*" {
  capabilities =  ["read", "update"]
}