#!/bin/bash

echo "Enter plugin name: "
read TRC_PLUGIN_NAME

if [ "$TRC_PLUGIN_NAME" = 'trc-vault-carrier-plugin' ] ; then
    echo "Use refreshcarriertoken to refresh carrier tokens."
    exit 1
fi

echo "Enter environment: "
read VAULT_ENV

if [[ -z "${SECRET_VAULT_ADDR}" ]]; then
echo "Enter organization vault host base url including port (hit enter if just refreshing org tokens): "
read SECRET_VAULT_ADDR
fi

if [[ -z "${SECRET_ENV_TOKEN}" ]]; then
echo "Enter organization vault *plugin* environment token with tightly confined write permissions(config_token_plugin$VAULT_ENV): "
read SECRET_ENV_TOKEN
fi

if [[ -z "${VAULT_ADDR}" ]]; then
echo "Enter agent vault host base url including port: "
read VAULT_ADDR
fi

if [[ -z "${VAULT_TOKEN}" ]]; then
echo "Enter agent vault root token: "
read VAULT_TOKEN
fi

echo "Enter organization vault unrestricted environment token with write permissions(config_token_"$VAULT_ENV"_unrestricted): "
read VAULT_ENV_TOKEN

VAULT_API_ADDR=$VAULT_ADDR
VAULT_API_TOKEN=$VAULT_TOKEN
export VAULT_ADDR
export VAULT_TOKEN
export VAULT_API_ADDR
export VAULT_API_TOKEN

echo "Secret vault: " $SECRET_VAULT_ADDR
echo "Agent vault: " $VAULT_ADDR

echo "Agent and secrets stored in secrets vault only? (Y or N): "
read SECRETS_ONLY

if [ "$SECRETS_ONLY" = "Y" ] || [ "$SECRETS_ONLY" = "yes" ] || [ "$SECRETS_ONLY" = "y" ]; then
    VAULT_API_ADDR=$SECRET_VAULT_ADDR
    VAULT_API_TOKEN=$SECRET_ENV_TOKEN
fi

vault write $TRC_PLUGIN_NAME/$VAULT_ENV token=$VAULT_API_TOKEN vaddress=$VAULT_API_ADDR caddress=$SECRET_VAULT_ADDR ctoken=$SECRET_ENV_TOKEN plugin=$TRC_PLUGIN_NAME

