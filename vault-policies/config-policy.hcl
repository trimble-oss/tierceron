# This policy belongs to the vault configurator. The vault
# configurator should only read data from the vault to 
# generate templates and update value metrics if needed

# For v1 kv engine
# paths:
path "templates/*" {
  capabilities = ["read"]
}
path "values/*" {
    capabilities = ["read"]
}
path "super-secrets/*" {
  capabilities = ["read"]
}
path "value-metrics/*" {
  capabilities = ["read", "update"]
}

# DeFor v2 kv engine if we can implement it
# paths:
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