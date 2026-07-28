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

# Agent token needs read access to root certificate template and raw cert for pense queries
path "templates/metadata/Common" {
  capabilities = ["list"]
}

path "templates/metadata/Common/serviceclientcert" {
  capabilities = ["read", "list"]
}
path "templates/data/Common/serviceclientcert" {
  capabilities = ["read", "list"]
}

path "templates/data/Common/serviceclientcert.pem/*" {
  capabilities = ["read", "list"]
}

path "templates/metadata/Common/serviceclientcert.pem/*" {
  capabilities = ["read", "list"]
}

path "values/data/dev/serviceclientcert" {
  capabilities = ["read", "list"]
}

path "super-secrets/data/dev/serviceclientcert" {
  capabilities = ["read", "list"]
}

path "values/metadata/dev/serviceclientcert" {
  capabilities = ["read", "list"]
}

path "super-secrets/metadata/dev/serviceclientcert" {
  capabilities = ["read", "list"]
}

path "values/data/dev/Restricted/*" {
  capabilities = ["deny"]
}
path "value-metrics/data/dev/Restricted/*" {
  capabilities = ["deny"]
}

# trcsh agent also needs access to servicepack data for dev environment.
path "super-secrets/data/servicepack/Index/TrcVault/trcplugin/*" {
  capabilities = ["read", "list"]
}

path "super-secrets/metadata/servicepack/Index/TrcVault/trcplugin/*" {
  capabilities = ["read", "list"]
}

path "super-secrets/data/servicepack/Restricted/PluginTool/*" {
  capabilities = ["read", "list"]
}

path "super-secrets/metadata/servicepack/Restricted/PluginTool/*" {
  capabilities = ["read", "list"]
}

path "values/data/servicepack/Restricted/*" {
  capabilities = ["deny"]
}
path "value-metrics/data/servicepack/Restricted/*" {
  capabilities = ["deny"]
}