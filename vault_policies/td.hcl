# This policy belongs to the tder

# For v1 kv engine
# paths:
path "templates/ST/config/template-file" {
  capabilities = ["create"]
}

# DeFor v2 kv engine if we can implement it
# paths:
path "data/templates/ST/config/template-file" {
  capabilities = ["create"]
}
