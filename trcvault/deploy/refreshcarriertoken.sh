#!/bin/bash
if [[ -z "${VAULT_ADDR}" ]]; then
echo "Enter agent vault host base url including port: "
read VAULT_ADDR
fi

if [[ -z "${SECRET_VAULT_ADDR}" ]]; then
<<<<<<< HEAD
echo "Enter organization vault host base url including port (hit enter if just refreshing org tokens): "
=======
echo "Enter secrets vault host base url including port: "
>>>>>>> releases/comprehensive/1.0.0
read SECRET_VAULT_ADDR
fi

if [[ -z "${VAULT_TOKEN}" ]]; then
echo "Enter agent vault root token: "
read VAULT_TOKEN
fi

if [[ -z "${VAULT_ENV}" ]]; then
<<<<<<< HEAD
echo "Enter vault environment being configured: "
=======
echo "Enter agent and secrets common vault environment: "
>>>>>>> releases/comprehensive/1.0.0
read VAULT_ENV
fi

if [[ -z "${VAULT_ENV_TOKEN}" ]]; then
<<<<<<< HEAD
echo "Enter organization vault unrestricted environment token with write permissions: "
=======
echo "Enter agent unrestricted environment token with write permissions: "
>>>>>>> releases/comprehensive/1.0.0
read VAULT_ENV_TOKEN
fi


if [[ -z "${SECRET_VAULT_CONFIG_ROLE}" ]]; then
<<<<<<< HEAD
echo "Enter organization vault config approle: "
=======
echo "Enter secrets vault config approle: "
>>>>>>> releases/comprehensive/1.0.0
read SECRET_VAULT_CONFIG_ROLE
fi

if [[ -z "${SECRET_VAULT_PUB_ROLE}" ]]; then
<<<<<<< HEAD
echo "Enter organization vault config pubrole: "
=======
echo "Enter secrets vault config pubrole: "
>>>>>>> releases/comprehensive/1.0.0
read SECRET_VAULT_PUB_ROLE
fi

if [[ -z "${KUBE_PATH}" ]]; then
echo "Enter organization kube config path: "
read KUBE_PATH
fi

export TRC_KUBE_CONFIG=`cat $KUBE_PATH | base64 --wrap=0`

VAULT_API_ADDR=VAULT_ADDR
export VAULT_ADDR
export VAULT_TOKEN
export VAULT_API_ADDR

echo $VAULT_ADDR

vault write vaultcarrier/$VAULT_ENV token=$VAULT_ENV_TOKEN vaddress=$SECRET_VAULT_ADDR pubrole=$SECRET_VAULT_PUB_ROLE configrole=$SECRET_VAULT_CONFIG_ROLE kubeconfig=$TRC_KUBE_CONFIG

