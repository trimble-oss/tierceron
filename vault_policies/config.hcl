#----------------------------------------------------#
#                        Local                       #
#----------------------------------------------------#
path "local/data/*" {
  capabilities = ["read", "list"]
}

path "local/metadata/*" {
  capabilities = ["read", "list"]
}

path "local/data/value-metrics/*" {
  capabilities = ["read", "list", "create", "update"]
}

#----------------------------------------------------#
#                        Dev                         #
#----------------------------------------------------#
path "dev/data/*" {
  capabilities = ["read", "list"]
}

path "dev/metadata/*" {
  capabilities = ["read", "list"]
}

path "dev/data/value-metrics/*" {
  capabilities = ["read", "list", "create", "update"]
}

#----------------------------------------------------#
#                        QA                          #
#----------------------------------------------------#
path "QA/data/*" {
  capabilities = ["read", "list"]
}

path "QA/metadata/*" {
  capabilities = ["read", "list"]
}

path "QA/data/value-metrics/*" {
  capabilities = ["read", "list", "create", "update"]
}

#----------------------------------------------------#
#                 Default (secrets)                  #
#----------------------------------------------------#
path "secret/data/*" {
  capabilities = ["read", "list"]
}

path "secret/metadata/*" {
  capabilities = ["read", "list"]
}

path "secret/data/value-metrics/*" {
  capabilities = ["read", "list", "create", "update"]
}
