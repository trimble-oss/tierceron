#!/bin/bash

echo "Enter plugin name: "
read TRC_PLUGIN_NAME

if [ "$TRC_PLUGIN_NAME" = 'trc-vault-carrier-plugin' ] ; then
    echo "Use refreshcarriertoken to refresh carrier tokens."
    exit 1
fi

echo "Enter organization vault host base url including port: "
read VAULT_ADDR

echo "Enter organization vault root token: "
read VAULT_TOKEN

echo "Enter environment: "
read VAULT_ENV

echo "Enter organization vault unrestricted environment token with write permissions: "
read VAULT_ENV_TOKEN

VAULT_API_ADDR=VAULT_ADDR
export VAULT_ADDR
export VAULT_TOKEN
export VAULT_API_ADDR

vault write $TRC_PLUGIN_NAME/$VAULT_ENV token=$VAULT_ENV_TOKEN vaddress=$VAULT_ADDR

