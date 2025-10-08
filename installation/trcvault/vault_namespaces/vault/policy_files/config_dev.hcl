path "templates/*" {
  capabilities = ["read", "list"]
}
path "templates/metadata" {
  capabilities = ["list"]
}
path "values/metadata/dev/*" {
  capabilities = ["read", "list"]
}

path "values/data/dev/*" {
  capabilities = ["read", "list"]
}

path "super-secrets/metadata/dev/*" {
  capabilities = ["read", "list"]
}

path "super-secrets/data/dev/*" {
  capabilities = ["read", "list"]
}

path "super-secrets/data/dev/Restricted/PluginTool/*" {
  capabilities = ["read"]
}

path "super-secrets/data/dev/Index/TrcVault/trcplugin/overrides/*" {
  capabilities = ["create", "update", "read", "list"]
}

path "super-secrets/data/dev/Index/TrcVault/trcplugin/trcctl/*" {
  capabilities = ["read", "list"]
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

path "super-secrets/data/dev-*" {
  capabilities = ["read", "list"]
}

path "super-secrets/metadata/dev-*" {
  capabilities = ["read", "list"]
}

path "values/data/dev-*" {
  capabilities = ["read", "list"]
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
  capabilities = ["deny"]
}
path "values/data/dev/Restricted/*" {
  capabilities = ["deny"]
}
path "super-secrets/metadata/dev/Restricted/*" {
  capabilities = ["deny"]
}
path "super-secrets/data/dev/Restricted/*" {
  capabilities = ["deny"]
}
path "value-metrics/dev/Restricted/*" {
  capabilities = ["deny"]
}
path "super-secrets/metadata/dev/Restricted/*" {
  capabilities = ["deny"]
}

path "values/metadata/dev-*/Restricted/*" {
  capabilities = ["deny"]
}
path "values/data/dev-*/Restricted/*" {
  capabilities = ["deny"]
}
path "super-secrets/metadata/dev-*/Restricted/*" {
  capabilities = ["deny"]
}
path "super-secrets/data/dev-*/Restricted/*" {
  capabilities = ["deny"]
}
path "value-metrics/dev-*/Restricted/*" {
  capabilities = ["deny"]
}
path "super-secrets/metadata/dev-*/Restricted/*" {
  capabilities = ["deny"]
}

