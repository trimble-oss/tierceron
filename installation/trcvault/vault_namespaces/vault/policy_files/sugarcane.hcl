path "super-secrets/data/sugarcane/*" {
    capabilities=["read", "list"]
}

path "values/metadata/*" {
  capabilities = ["deny"]
}

path "values/data/*" {
  capabilities = ["deny"]
}
