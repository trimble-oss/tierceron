path "templates/*" {
  capabilities = ["read", "list", "create", "update"]
}
path "templates/metadata" {
  capabilities = ["list"]
}
path "values/metadata/dev/*" {
  capabilities = ["read", "list"]
}

path "values/data/dev/*" {
  capabilities = ["read", "list", "create", "update"]
}

path "super-secrets/metadata/dev/*" {
  capabilities = ["read", "list"]
}

path "super-secrets/data/dev/*" {
  capabilities = ["read", "list", "create", "update"]
}

path "value-metrics/data/dev/*" {
  capabilities = ["read", "list", "create", "update"]
}
path "value-metrics/dev/*" {
  capabilities = ["read", "list", "create", "update"]
}

path "values/metadata" {
  capabilities = ["list"]
}

path "super-secrets/metadata/dev" {
  capabilities = ["read", "list"]
}

path "super-secrets/metadata" {
  capabilities = ["list"]
}

# Adding a dev-* section to support 2+ configurations for dev
path "super-secrets/data/dev-*" {
  capabilities = ["read", "list", "create", "update"]
}

path "super-secrets/metadata/dev-*" {
  capabilities = ["read", "list"]
}

path "values/data/dev-*" {
  capabilities = ["read", "list", "create", "update"]
}

path "values/metadata/dev-*" {
  capabilities = ["read", "list"]
}

path "value-metrics/dev-*" {
  capabilities = ["read", "list", "create", "update"]
}

path "value-metrics/data/dev-*" {
  capabilities = ["read", "list", "create", "update"]
}

# Adding a restricted section
# Only a special token can access the restricted section.
path "values/metadata/dev/Restricted/*" {
  capabilities = ["create", "update", "list", "read"]
}
path "values/data/dev/Restricted/*" {
  capabilities = ["create", "update", "list", "read"]
}
path "super-secrets/metadata/dev/Restricted/*" {
  capabilities = ["create", "update", "list", "read"]
}
path "super-secrets/data/dev/Restricted/*" {
  capabilities = ["create", "update", "list", "read"]
}
path "value-metrics/data/dev/Restricted/*" {
  capabilities = ["create", "update", "list", "read"]
}
path "value-metrics/dev/Restricted/*" {
  capabilities = ["create", "update", "list", "read"]
}
path "super-secrets/metadata/dev/Restricted/*" {
  capabilities = ["create", "update", "list", "read"]
}
