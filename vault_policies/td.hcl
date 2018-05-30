# This policy belongs to the tder

# For v1 kv engine
# paths:
path "templates/ST/config/template-file" {
  capabilities = ["create", "update"]
}

path "templates/ST/hibernate/template-file" {
  capabilities = ["create", "update"]
}

# eFor v2 kv engine if we can implement it
# paths:
path "data/templates/ST/config/template-file" {
  capabilities = ["create", "update"]
}

path "data/templates/ST/hibernate/template-file" {
  capabilities = ["create", "update"]
}
