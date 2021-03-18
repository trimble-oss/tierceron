path "auth/token/*" {
  capabilities = [ "create", "update", "read" ]
}

path "sys/policy/rest_integration_consumer_*" {
  capabilities = [ "create", "update", "read" ]
}