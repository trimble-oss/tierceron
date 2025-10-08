path "super-secrets/data/pub/*" {
    capabilities=["read", "list"]
}

path "values/metadata/*" {
  capabilities = ["deny"]
}

path "values/data/*" {
  capabilities = ["deny"]
}
