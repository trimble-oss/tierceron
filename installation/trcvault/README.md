# Introduction 
The installation folder for trcdb.  This is responsible for setting up the initial tokens you'll
need to manage your vault.

# Prerequisites
This assumes the existence of a machine or virtual machine (in the cloud) with a reachable ip address
and host name.

# Build initial vault.
Current version of vault: vault 1.3.6 (downloadable here: https://releases.hashicorp.com/vault/1.3.6/)

trcinit -new -namespace=vault -addr=https://<vaulthost:vaultport> -totalKeys=3 -unsealKeys=2 > tokens.txt

# Additional helpful commands
For reference, the default token namespaces provided are as follows..
Namespaces:
agent - Tokens for deployment agents.
vault - Env based tokens.

# Get expiration for existing tokens in provided namespace.
trcinit -tokenExpiration -namespace=vault -addr=https://<vaulthost:vaultport> -token=$TRC_ROOT_TOKEN

# Rotate tokens in provided namespace.
trcinit -rotateTokens -namespace=rest -addr=https://<vaulthost:vaultport> -token=$TRC_ROOT_TOKEN


