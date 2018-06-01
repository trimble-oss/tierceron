#----------------------------------------------------#
#                        Local                       #
#----------------------------------------------------#
path "local/data/templates/ST/config/template-file" {
  capabilities = ["create", "update"]
}

path "local/data/templates/ST/hibernate/template-file" {
  capabilities = ["create", "update"]
}

path "localt/data/values" {
  capabilites = ["create", "update"]
}

path "local/data/value-metrics" {
  capabilites = ["create", "update"]
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

path "dev/data/values" {
  capabilites = ["create", "update"]
}

path "dev/data/value-metrics" {
  capabilites = ["create", "update"]
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

path "QA/data/values" {
  capabilites = ["create", "update"]
}

path "QA/data/value-metrics" {
  capabilites = ["create", "update"]
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

path "secret/data/values" {
  capabilites = ["create", "update"]
}

path "secret/data/value-metrics" {
  capabilites = ["create", "update"]
}