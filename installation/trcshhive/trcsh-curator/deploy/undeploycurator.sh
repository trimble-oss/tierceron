#!/bin/bash

if [[ -z "${VAULT_ADDR}" ]]; then
echo "Enter vault host base url: "
read VAULT_ADDR
fi

if [[ -z "${VAULT_TOKEN}" ]]; then
echo "Enter root token: "
read VAULT_TOKEN
fi

export VAULT_ADDR
export VAULT_TOKEN

vault secrets disable vaultcurator/
vault secrets list | grep trcsh-curator$PROD_EXT
existingplugin=$?
if [ $existingplugin -eq 0 ]; then       
    echo "Carrier plugin still mounted elsewhere.  Manual intervention required to clean up before deregistration can proceed."
    exit 1
else
    echo "All mounts cleared.  Continuing..."
fi
vault plugin deregister secret trcsh-curator$PROD_EXT
