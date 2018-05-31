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