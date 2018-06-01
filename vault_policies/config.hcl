#----------------------------------------------------#
#                        Local                       #
#----------------------------------------------------#
path "local/data/templates/*" {
  capabilities = ["read"]
}
path "local/data/values/*" {
    capabilities = ["read"]
}
path "local/data/super-secrets/*" {
  capabilities = ["read"]
}
path "local/data/value-metrics/*" {
  capabilities =  ["read", "create", "update"]
}

#----------------------------------------------------#
#                        Dev                         #
#----------------------------------------------------#
path "dev/data/templates/*" {
  capabilities = ["read"]
}
path "dev/data/values/*" {
    capabilities = ["read"]
}
path "dev/data/super-secrets/*" {
  capabilities = ["read"]
}
path "dev/data/value-metrics/*" {
  capabilities =  ["read", "create", "update"]
}

#----------------------------------------------------#
#                        QA                          #
#----------------------------------------------------#
path "QA/data/templates/*" {
  capabilities = ["read"]
}
path "QA/data/values/*" {
    capabilities = ["read"]
}
path "QA/data/super-secrets/*" {
  capabilities = ["read"]
}
path "QA/data/value-metrics/*" {
  capabilities =  ["read", "create", "update"]
}

#----------------------------------------------------#
#                 Default (secrets)                  #
#----------------------------------------------------#

path "secret/data/templates/*" {
  capabilities = ["read"]
}
path "secret/data/values/*" {
    capabilities = ["read"]
}
path "secret/data/super-secrets/*" {
  capabilities = ["read"]
}
path "secret/data/value-metrics/*" {
  capabilities =  ["read", "create", "update"]
}