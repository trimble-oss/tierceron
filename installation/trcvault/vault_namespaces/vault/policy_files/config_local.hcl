path "templates/*" {
  capabilities = ["read", "list"]
}
path "templates/metadata" {
  capabilities = ["list"]
}
path "values/metadata/local/*" {
  capabilities = ["read", "list"]
}

path "values/data/local/*" {
  capabilities = ["read", "list"]
}

path "super-secrets/metadata/local/*" {
  capabilities = ["read", "list"]
}

path "super-secrets/data/local/*" {
  capabilities = ["read", "list"]
}

path "value-metrics/local/*" {
  capabilities = ["read", "list", "create", "update"]
}

# Adding a restricted section
# Only a special token can access the restricted section.
path "values/metadata/local/Restricted/*" {
  capabilities = ["deny"]
}
path "values/data/local/Restricted/*" {
  capabilities = ["deny"]
}
path "super-secrets/metadata/local/Restricted/*" {
  capabilities = ["deny"]
}
path "super-secrets/data/local/Restricted/*" {
  capabilities = ["deny"]
}
path "value-metrics/local/Restricted/*" {
  capabilities = ["deny"]
}
path "super-secrets/metadata/local/Restricted/*" {
  capabilities = ["deny"]
}
