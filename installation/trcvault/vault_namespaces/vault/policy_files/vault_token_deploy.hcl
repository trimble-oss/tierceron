#Denying everything but deploy directory
path "*" {
  capabilities = ["deny"]
}

path "vaultcarrier/deploy/*" {
  capabilities = ["read", "list"]
}
