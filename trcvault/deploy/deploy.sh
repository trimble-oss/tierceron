#!/bin/bash

if [[ -z "${VAULT_ADDR}" ]]; then
echo "Enter vault host base url: "
read VAULT_ADDR
fi

if [[ -z "${VAULT_TOKEN}" ]]; then
echo "Enter root token: "
read VAULT_TOKEN
fi

if [[ -z "${VAULT_ENV}" ]]; then
echo "Enter environment: "
read VAULT_ENV
fi

if [[ -z "${VAULT_ENV_TOKEN}" ]]; then
echo "Enter environment token with write permissions: "
read VAULT_ENV_TOKEN
fi

vault secrets disable vaultdb/
vault plugin deregister trc-vault-plugin

if [ "$VAULT_ENV" = "prod" ] || [ "$VAULT_ENV" = "staging" ]
then
# Just writing to vault will trigger the carrier plugin...
# First we set Copied to false...
# This should also trigger the copy process...
# It should return sha256 of copied plugin on success.
SHA256BUNDLE=$(vault write vaultcarrier/$VAULT_ENV token=$VAULT_ENV_TOKEN plugin=trc-vault-plugin-prod)
SHAVAL=$(echo $SHA256BUNDLE | awk '{print $6}')

vault plugin register \
          -command=trc-vault-plugin-prod \
          -sha256=$SHAVAL \
          -args=`backendUUID=4` \
          trc-vault-plugin-prod
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

# TODO: If this errors, then fail...
vault plugin register \
          -command=trc-vault-plugin \
          -sha256=$SHAVAL \
          -args=`backendUUID=4` \
          trc-vault-plugin
vault secrets enable \
          -path=vaultdb \
          -plugin-name=trc-vault-plugin \
          -description="Tierceron Vault Plugin" \
          plugin
fi

#Activates/starts the deployed plugin.
# Note: plugin should update deployed flag for itself.
vault write vaultdb/$VAULT_ENV token=$VAULT_ENV_TOKEN

