# Create trcshhivez AppRole (read-only access)
vault write auth/approle/role/trcshhivez \
    token_policies="trcshhivez" \
    token_ttl=8h \
    token_max_ttl=24h \
    secret_id_ttl=0 \
    secret_id_num_uses=0

# Create trcshunrestricted AppRole (write access)  
vault write auth/approle/role/trcshunrestricted \
    token_policies="trcshunrestricted" \
    token_ttl=8h \
    token_max_ttl=24h \
    secret_id_ttl=0 \
    secret_id_num_uses=0