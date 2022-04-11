#!/bin/bash

echo "Enter vault host base url: "
read VAULT_ADDR

echo "Enter root token: "
read VAULT_TOKEN

echo "Enter environment: "
read VAULT_ENV

echo "Enter environment token with write permissions: "
read VAULT_ENV_TOKEN

VAULT_API_ADDR=VAULT_ADDR
export VAULT_ADDR
export VAULT_API_ADDR

echo "Disable old trc vault secrets"
vault secrets disable vaultdb/
echo "Unregister old trc vault plugin"
vault plugin deregister trc-vault-plugin

if [ "$VAULT_ENV" = "prod" ] || [ "$VAULT_ENV" = "staging" ]
then
# Just writing to vault will trigger the carrier plugin...
# First we set Copied to false...
# This should also trigger the copy process...
# It should return sha256 of copied plugin on success.
SHA256BUNDLE=$(vault write vaultcarrier/$VAULT_ENV token=$VAULT_ENV_TOKEN plugin=trc-vault-plugin-prod)
SHAVAL=$(echo $SHA256BUNDLE | awk '{print $6}')

echo "Registering new plugin."
vault plugin register \
          -command=trc-vault-plugin-prod \
          -sha256=$(echo $SHAVAL) \
          -args=`backendUUID=789` \
          trc-vault-plugin-prod
echo "Enabling new plugin."
vault secrets enable \
          -path=vaultdb \
          -plugin-name=trc-vault-plugin-prod \
          -description="Tierceron Vault Plugin Prod" \
          plugin
else
# Just writing to vault will trigger a copy...
# First we set Copied to false...
# This should also trigger the copy process...
# It should return sha256 of copied plugin on success.
SHA256BUNDLE=$(vault write vaultcarrier/$VAULT_ENV token=$VAULT_ENV_TOKEN plugin=trc-vault-plugin)
SHAVAL=$(echo $SHA256BUNDLE | awk '{print $6}')

echo "Registering new plugin."
vault plugin register \
          -command=trc-vault-plugin \
          -sha256=$(echo $SHAVAL) \
          -args=`backendUUID=789` \
          trc-vault-plugin
echo "Enabling new plugin."
vault secrets enable \
          -path=vaultdb \
          -plugin-name=trc-vault-plugin \
          -description="Tierceron Vault Plugin" \
          plugin
fi

#Activates/starts the deployed plugin.
# Note: plugin should update deployed flag for itself.
vault write vaultdb/$VAULT_ENV token=$VAULT_ENV_TOKEN
