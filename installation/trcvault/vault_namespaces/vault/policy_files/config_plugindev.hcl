path "templates/*" {
  capabilities = ["read", "list"]
}

path "templates/metadata" {
  capabilities = ["list"]
}

path "super-secrets/data/azuredeploy/*" {
    capabilities=["read", "list"]
}

path "values/metadata/dev/*" {
  capabilities = ["read", "list"]
}

path "values/data/dev/*" {
  capabilities = ["read", "list"]
}

path "values/data/dev/Index/*" {
  capabilities = ["create", "update", "read", "list"]
}

path "values/metadata" {
  capabilities = ["list"]
}

path "super-secrets/metadata" {
  capabilities = ["list"]
}

path "super-secrets/metadata/dev" {
  capabilities = ["read", "list"]
}

path "super-secrets/metadata/dev/*" {
  capabilities = ["read", "list"]
}

path "super-secrets/data/dev/*" {
  capabilities = ["list"]
}

path "super-secrets/data/dev/Index/TrcVault/trcplugin/*" {
  capabilities = ["create", "update", "read", "list"]
}

path "super-secrets/data/dev/TierceronFlow" {
  capabilities = ["read", "list"]
}

path "super-secrets/data/dev/DataFlowStatistics" {
  capabilities = ["read", "list"]
}
path "super-secrets/data/dev/Index/FlumeDatabase/*" {
  capabilities = ["read", "list"]
}

path "super-secrets/data/dev/Index/TenantDatabase/*" {
  capabilities = ["read", "list"]
}

path "super-secrets/data/dev/PublicIndex/TenantDatabase/*" {
  capabilities = ["create", "update", "read", "list"]
}

#This is for access to updating the APIM in trcsh
path "super-secrets/data/dev/Restricted/APIMConfig/*" {
  capabilities = ["read", "list"]
}

#This is for cursor deployments in trcsh
path "super-secrets/data/dev/Restricted/TrcshCursor*" {
  capabilities = ["read", "list"]
}

path "value-metrics/data/dev/*" {
  capabilities = ["read", "list", "create", "update"]
}

# Adding a restricted section
# Only a special token can access the restricted section.
path "values/metadata/dev/Restricted/*" {
  capabilities = ["read"]
}
path "values/data/dev/Restricted/*" {
  capabilities = ["read"]
}
path "super-secrets/metadata/dev/Restricted/*" {
  capabilities = ["read"]
}
path "super-secrets/data/dev/Restricted/PluginTool/*" {
  capabilities = ["read"]
}
path "value-metrics/dev/Restricted/*" {
  capabilities = ["read"]
}
path "super-secrets/metadata/dev/Restricted/*" {
  capabilities = ["read"]
}
