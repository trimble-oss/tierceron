path "super-secrets/data/deploy/*" {
    capabilities=["read", "list"]
}

path "super-secrets/data/azuredeploy/*" {
    capabilities=["read", "list"]
}

path "values/metadata/*" {
  capabilities = ["deny"]
}

path "values/data/*" {
  capabilities = ["deny"]
}
