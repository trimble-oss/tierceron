# Introduction 
The installation folder for trcvault.  This is responsible for setting up the initial tokens you'll need to manage your vault.

# Prerequisites
This assumes the existence of a running vault on your local machine or virtual machine (in the cloud) with a reachable ip address and host name running vault.  You should have run either trclocal or trccloud prior to this.

# network setup


# Build initial vault
```
trcinit -new -namespace=vault -addr=https://<vaulthost:vaultport> -totalKeys=3 -unsealKeys=2 > tokens.txt
```

Note, for local development installs where you may be using a self signed certificate, you can use the --insecure flag to give "flexibility" on certificate validation, meaning you want encrypted communication, but you are flexible on the validation of the certificate.

# Unseal vault (any time you restart vault)
/usr/local/vault/vault operator unseal -address="https://<vaulthost:vaultport>" -tls-skip-verify

# Additional helpful commands
For reference, the default token namespaces provided are as follows..
Namespaces:
agent - Tokens for deployment agents.
vault - Env based tokens.

# Get expiration for existing tokens in provided namespace
```
trcinit -tokenExpiration -namespace=vault -addr=https://<vaulthost:vaultport> -token=$TRC_ROOT_TOKEN
```

# Rotate tokens in provided namespace
```
trcinit -rotateTokens -namespace=vault -addr=https://<vaulthost:vaultport> -token=$TRC_ROOT_TOKEN
```

# Update roles
```
trcinit -updateRole -namespace=vault -addr=https://<vaulthost:vaultport> -token=$TRC_ROOT_TOKEN
```

# Update policies
```
trcinit -updatePolicy -namespace=vault -addr=https://<vaulthost:vaultport> -token=$TRC_ROOT_TOKEN
```
