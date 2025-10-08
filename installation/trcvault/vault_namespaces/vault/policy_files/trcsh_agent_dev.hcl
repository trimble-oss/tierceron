path "super-secrets/data/dev/Index/TrcVault/trcplugin/*" {
  capabilities = ["read", "list"]
}

path "super-secrets/metadata/dev/Index/TrcVault/trcplugin/*" {
  capabilities = ["read", "list"]
}

path "super-secrets/data/dev/Restricted/PluginTool/*" {
  capabilities = ["read", "list"]
}

path "super-secrets/metadata/dev/Restricted/PluginTool/*" {
  capabilities = ["read", "list"]
}

path "super-secrets/data/dev/Restricted/TrcshCursor*" {
  capabilities = ["read", "list"]
}
path "super-secrets/metadata/dev/Restricted/TrcshCursor*" {
  capabilities = ["read", "list"]
}
path "values/data/dev/Restricted/*" {
  capabilities = ["deny"]
}
path "value-metrics/data/dev/Restricted/*" {
  capabilities = ["deny"]
}