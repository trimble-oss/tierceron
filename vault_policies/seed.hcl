# This policy belongs to the vault seeder. The vault seeder's
# sole purpose is to write the seed values to the vault

# For v1 kv engine
# paths:
path "templates/*" {
  capabilities = ["create", "read", "update"]
  required_parameters =  {
      "outputPath" = []
  }
}
path "values/*" {
  capabilities = ["create", "read", "update"]
}
path "super-secrets/*" {
  capabilities = ["create", "read", "update"]
}
path "value-metrics/*" {
  capabilities = ["create", "read", "update"]
}

# For v2 kv engine if we can implement it
# paths:
path "data/templates/*" {
  capabilities = ["create", "read", "update"]
  required_parameters  =  {
      "outputPath" = []
  }
}
path "data/values/*" {
  capabilities = ["create", "read", "update"]
}
path "data/super-secrets/*" {
  capabilities = ["create", "read", "update"]
}
path "data/value-metrics/*" {
  capabilities = ["create", "read", "update"]
}