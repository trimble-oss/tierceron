path "templates/*" {
    capabilities = ["read", "list"]
}

path "values/*" {
    capabilities = ["read", "list"]
}

path "value-metrics/*" {
    capabilities = ["read", "list"]
}

path "verification/*" {
    capabilities = ["read", "list"]
}

path "sys/policy" {
    capabilities = ["read", "list"]
}

path "sys/policy/*" {
    capabilities = ["read", "list"]
}

path "sys/auth/*" {
    capabilities = ["read", "list"]
}

path "sys/health" {
    capabilities = ["read", "sudo"]
}

path "sys/capabilities" {
  capabilities = ["create", "update"]
}

path "sys/capabilities-self" {
  capabilities = ["create", "update"]
}