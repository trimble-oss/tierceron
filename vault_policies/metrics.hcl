#----------------------------------------------------#
#                        Local                       #
#----------------------------------------------------#
path "local/data/templates/*" {
  capabilities = ["read"]
}

path "local/data/value-metrics/*" {
  capabilities = ["read", "update"]
}

#----------------------------------------------------#
#                        Dev                         #
#----------------------------------------------------#
path "dev/data/templates/*" {
  capabilities = ["read"]
}

path "dev/data/value-metrics/*" {
  capabilities = ["read", "update"]
}

#----------------------------------------------------#
#                        QA                          #
#----------------------------------------------------#
path "QA/data/templates/*" {
  capabilities = ["read"]
}

path "QA/data/value-metrics/*" {
  capabilities = ["read", "update"]
}

#----------------------------------------------------#
#                 Default (secrets)                  #
#----------------------------------------------------#

path "secret/data/templates/*" {
  capabilities = ["read"]
}

path "secret/data/value-metrics/*" {
  capabilities = ["read", "update"]
}
