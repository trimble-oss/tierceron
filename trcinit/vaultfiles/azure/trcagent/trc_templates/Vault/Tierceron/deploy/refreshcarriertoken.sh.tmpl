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


if [[ -z "${VAULT_CONFIG_ROLE}" ]]; then
echo "Enter config approle: "
read VAULT_CONFIG_ROLE
fi

if [[ -z "${VAULT_PUB_ROLE}" ]]; then
echo "Enter config pubrole: "
read VAULT_PUB_ROLE
fi

if [[ -z "${KUBE_PATH}" ]]; then
echo "Enter kube config path: "
read KUBE_PATH
fi

export VAULT_KUBE_CONFIG=`cat $KUBE_PATH | base64 --wrap=0`

VAULT_API_ADDR=VAULT_ADDR
export VAULT_ADDR
export VAULT_TOKEN
export VAULT_API_ADDR

echo $VAULT_ADDR

vault write vaultcarrier/$VAULT_ENV token=$VAULT_ENV_TOKEN vaddress=$VAULT_ADDR pubrole=$VAULT_PUB_ROLE configrole=$VAULT_CONFIG_ROLE kubeconfig=$VAULT_KUBE_CONFIG

