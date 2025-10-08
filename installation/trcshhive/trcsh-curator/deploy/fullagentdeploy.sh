#!/bin/bash

if [[ -z "${AGENT_VAULT_ADDR}" ]]; then
echo "Enter agent vault host base url: "
read AGENT_VAULT_ADDR
fi

if [[ ! "${AGENT_VAULT_ADDR}" == "https://"* ]]; then
echo "Agent vault host must begin with https:// "
exit -1
fi

if [[ -z "${AGENT_VAULT_TOKEN}" ]]; then
echo "Enter agent root token: "
read AGENT_VAULT_TOKEN
fi

VAULT_TOKEN=$AGENT_VAULT_TOKEN
VAULT_API_ADDR=$SECRET_VAULT_ADDR
VAULT_ADDR=$AGENT_VAULT_ADDR
SECRET_VAULT_ENV_TOKEN=$VAULT_ENV_TOKEN

export VAULT_ADDR
export VAULT_TOKEN
export VAULT_API_ADDR
export SECRET_VAULT_ENV_TOKEN

#================================================================
#trcshk deployment
#================================================================
echo "Deploying trcshk agent"
TRC_PLUGIN_NAME="trcshk"

FILE="target/$TRC_PLUGIN_NAME"
if [ ! -f "$FILE" ]; then
    echo "$FILE does not exist."
    exit 1
fi

FILESHA="target/$TRC_PLUGIN_NAME.sha256"
if [ ! -f "$FILESHA" ]; then
    echo "$FILESHA does not exist."
    exit 1
fi

FILESHAVAL=$(cat $FILESHA)

if [ -z "${PROD_EXT}" ]; then
    VAULT_ENV="dev"
    vault kv patch super-secrets/$VAULT_ENV/Index/TrcVault/trcplugin/$TRC_PLUGIN_NAME/Certify trcsha256=$FILESHAVAL
    VAULT_ENV="QA"
    vault kv patch super-secrets/$VAULT_ENV/Index/TrcVault/trcplugin/$TRC_PLUGIN_NAME/Certify trcsha256=$FILESHAVAL
else
    vault kv patch super-secrets/$VAULT_ENV/Index/TrcVault/trcplugin/$TRC_PLUGIN_NAME/Certify trcsha256=$FILESHAVAL
fi

echo "Deployed trcshk agent successfully"

#================================================================
#trcsh.exe deployment
#================================================================
echo "Deploying trcsh.exe agent"
TRC_PLUGIN_NAME="trcsh.exe"

FILE="target/$TRC_PLUGIN_NAME"
if [ ! -f "$FILE" ]; then
    echo "$FILE does not exist."
    exit 1
fi

FILESHA="target/$TRC_PLUGIN_NAME.sha256"
if [ ! -f "$FILESHA" ]; then
    echo "$FILESHA does not exist."
    exit 1
fi

FILESHAVAL=$(cat $FILESHA)

if [ -z "${PROD_EXT}" ]; then
    VAULT_ENV="dev"
    vault kv patch super-secrets/$VAULT_ENV/Index/TrcVault/trcplugin/$TRC_PLUGIN_NAME/Certify trcsha256=$FILESHAVAL
    VAULT_ENV="QA"
    vault kv patch super-secrets/$VAULT_ENV/Index/TrcVault/trcplugin/$TRC_PLUGIN_NAME/Certify trcsha256=$FILESHAVAL
    VAULT_ENV="RQA"
    vault kv patch super-secrets/$VAULT_ENV/Index/TrcVault/trcplugin/$TRC_PLUGIN_NAME/Certify trcsha256=$FILESHAVAL
    echo "Deployed trcsh.exe agent successfully"
else
    echo "Skipping $TRC_PLUGIN_NAME deployment in prod."
fi

exit 0
