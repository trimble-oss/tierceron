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


For reference, the default token namespaces provided are as follows..


Namespaces:
* agent - Tokens for deployment agents.
* vault - Env based tokens.


# Rebooting vault (requires unseal with a quorom of unseal keys)
You'll need to run the following command once for each unseal key you set up...

```
VAULT_ADDR=https://<vaulthost:vaultport> /usr/local/vault/vault operator unseal
```

Note, for local development installs where you may be using a self signed certificate, you can use the --tls-skip-verify

# Additional helpful commands
The commands below are helpful commands for managing tokens later on...

## Get expiration for existing tokens in provided namespace
```
trcinit -tokenExpiration -namespace=vault -addr=https://<vaulthost:vaultport> -token=$VAULT_TOKEN
```

## Rotate tokens in provided namespace
```
trcinit -rotateTokens -namespace=vault -addr=https://<vaulthost:vaultport> -token=$VAULT_TOKEN
```

The following creates roles for deploy and azuredeploy.
```
trcinit -rotateTokens -approle=deploy -namespace=agent -addr=https://<vaulthost:vaultport> -token=$VAULT_TOKEN
```

The following creates roles for deploy and hivekernel.
```
trcinit -rotateTokens -approle=hivekernel -namespace=agent -addr=https://<vaulthost:vaultport> -token=$VAULTOKEN
```

## Update roles
```
trcinit -updateRole -namespace=vault -addr=https://<vaulthost:vaultport> -token=$VAULT_TOKEN
```

## Update policies
```
trcinit -updatePolicy -namespace=vault -addr=https://<vaulthost:vaultport> -token=$VAULT_TOKEN
```
