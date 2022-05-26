#!/bin/bash

echo "Enter plugin name: "
read TRC_PLUGIN_NAME

echo "Enter vault host base url: "
read VAULT_ADDR

echo "Enter root token: "
read VAULT_TOKEN

echo "Enter environment: "
read VAULT_ENV

echo "Enter trc plugin runtime environment token with write permissions unrestricted: "
read VAULT_ENV_TOKEN

VAULT_API_ADDR=VAULT_ADDR
export VAULT_ADDR
export VAULT_TOKEN
export VAULT_API_ADDR

vault write $TRC_PLUGIN_NAME/$VAULT_ENV token=$VAULT_ENV_TOKEN vaddress=$VAULT_ADDR

