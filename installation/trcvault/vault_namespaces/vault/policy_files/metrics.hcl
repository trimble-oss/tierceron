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
#                        RQA                         #
#----------------------------------------------------#
path "RQA/data/templates/*" {
  capabilities = ["read"]
}

path "RQA/data/value-metrics/*" {
  capabilities = ["read", "update"]
}

#----------------------------------------------------#
#                        auto                         #
#----------------------------------------------------#
path "auto/data/templates/*" {
  capabilities = ["read"]
}

path "auto/data/value-metrics/*" {
  capabilities = ["read", "update"]
}

#----------------------------------------------------#
#                        performance                 #
#----------------------------------------------------#
path "perfromance/data/templates/*" {
  capabilities = ["read"]
}

path "perfromance/data/value-metrics/*" {
  capabilities = ["read", "update"]
}


#----------------------------------------------------#
#                        Itdev                       #
#----------------------------------------------------#
path "itdev/data/templates/*" {
  capabilities = ["read"]
}

path "itdev/data/value-metrics/*" {
  capabilities = ["read", "update"]
}

#----------------------------------------------------#
#                        Servicepack                       #
#----------------------------------------------------#
path "servicepack/data/templates/*" {
  capabilities = ["read"]
}

path "servicepack/data/value-metrics/*" {
  capabilities = ["read", "update"]
}

#----------------------------------------------------#
#                        Staging                     #
#----------------------------------------------------#
path "staging/data/templates/*" {
  capabilities = ["read"]
}

path "staging/data/value-metrics/*" {
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
