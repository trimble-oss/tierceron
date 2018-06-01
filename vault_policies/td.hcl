#----------------------------------------------------#
#                        Local                       #
#----------------------------------------------------#
path "local/data/templates/ST/config/template-file" {
  capabilities = ["create", "update"]
}

path "local/data/templates/ST/hibernate/template-file" {
  capabilities = ["create", "update"]
}

path "local/data/values/*" {
  capabilities = ["create", "update"]
}

path "local/data/value-metrics/*" {
  capabilities = ["create", "update"]
}

#----------------------------------------------------#
#                        Dev                         #
#----------------------------------------------------#
path "dev/data/templates/ST/config/template-file" {
  capabilities = ["create", "update"]
}

path "dev/data/templates/ST/hibernate/template-file" {
  capabilities = ["create", "update"]
}

path "dev/data/values/*" {
  capabilities = ["create", "update"]
}

path "dev/data/value-metrics/*" {
  capabilities = ["create", "update"]
}

#----------------------------------------------------#
#                        QA                          #
#----------------------------------------------------#
path "QA/data/templates/ST/config/template-file" {
  capabilities = ["create", "update"]
}

path "QA/data/templates/ST/hibernate/template-file" {
  capabilities = ["create", "update"]
}

path "QA/data/values/*" {
  capabilities = ["create", "update"]
}

path "QA/data/value-metrics/*" {
  capabilities = ["create", "update"]
}

#----------------------------------------------------#
#                 Default (secrets)                  #
#----------------------------------------------------#
path "secret/data/templates/ST/config/template-file" {
  capabilities = ["create", "update"]
}

path "secret/data/templates/ST/hibernate/template-file" {
  capabilities = ["create", "update"]
}

path "secret/data/values/*" {
  capabilities = ["create", "update"]
}

path "secret/data/value-metrics/*" {
  capabilities = ["create", "update"]
}