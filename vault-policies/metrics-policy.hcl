# Low access policy for fetching metrics and 
# and template information, but no editing rights

# For v1 kv engine
# paths:
path "templates/*" {
  capabilities = ["read"]
}

path "value-metrics/*" {
  capabilities = ["read", "update"]
}

# For v2 kv engine
# paths:
path "data/templates/*" {
  capabilities = ["read"]
}

path "data/value-metrics/*" {
  capabilities = ["read", "update"]
}
