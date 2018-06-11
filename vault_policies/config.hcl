#----------------------------------------------------#
#                        Local                       #
#----------------------------------------------------#
path "local/data/*" {
  capabilities = ["read"]
}

path "local/metadata/*" {
  capabilities = ["read"]
}

path "local/data/value-metrics/*" {
  capabilities = ["read", "create", "update"]
}

#----------------------------------------------------#
#                        Dev                         #
#----------------------------------------------------#
path "dev/data/*" {
  capabilities = ["read"]
}

path "dev/metadata/*" {
  capabilities = ["read"]
}

path "dev/data/value-metrics/*" {
  capabilities = ["read", "create", "update"]
}

#----------------------------------------------------#
#                        QA                          #
#----------------------------------------------------#
path "QA/data/*" {
  capabilities = ["read"]
}

path "QA/metadata/*" {
  capabilities = ["read"]
}

path "QA/data/value-metrics/*" {
  capabilities = ["read", "create", "update"]
}

#----------------------------------------------------#
#                 Default (secrets)                  #
#----------------------------------------------------#
path "secret/data/*" {
  capabilities = ["read"]
}

path "secret/metadata/*" {
  capabilities = ["read"]
}

path "secret/data/value-metrics/*" {
  capabilities = ["read", "create", "update"]
}
