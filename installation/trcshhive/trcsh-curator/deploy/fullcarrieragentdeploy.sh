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

if [[ -z "${SECRET_VAULT_ADDR}" ]]; then
echo "Enter organization vault host base url including port (hit enter if just refreshing org tokens): "
read SECRET_VAULT_ADDR
fi

if [[ ! "${SECRET_VAULT_ADDR}" == "https://"* ]]; then
echo "Organization vault host must begin with https:// "
exit -1
fi


echo "Is this a prod environment? (Y or N): "
read PROD_ENV

if [ "$PROD_ENV" = 'Y' ] || [ "$PROD_ENV" = 'y' ]; then
    if [[ -z "${VAULT_ENV}" ]]; then
        echo "Enter organization/agent environment: "
        read VAULT_ENV
    fi

PROD_EXT=""
    for x in "staging" "prod"; do
        if [ $x = $VAULT_ENV ]; then
           PROD_EXT="-prod"
        fi
    done
fi

if [[ -z "${VAULT_ENV_TOKEN}" ]]; then
echo "Enter agent vault *plugin* environment token with tightly confined write permissions(config_token_pluginany): "
read VAULT_ENV_TOKEN
fi

if [ -z "${PROD_EXT}" ]; then
    if [[ -z "${SECRET_VAULT_DEV_PLUGIN_TOKEN}" ]]; then
    echo "Enter organization vault plugin token for certification(config_token_dev_unrestricted): "
    read SECRET_VAULT_DEV_PLUGIN_TOKEN
    fi

    if [[ -z "${SECRET_VAULT_QA_PLUGIN_TOKEN}" ]]; then
    echo "Enter organization vault plugin token for certification(config_token_QA_unrestricted): "
    read SECRET_VAULT_QA_PLUGIN_TOKEN
    fi
else
   if [[ -z "${SECRET_VAULT_PLUGIN_TOKEN}" ]]; then
    echo "Enter organization vault plugin token for certification(config_token_"$VAULT_ENV"_unrestricted): "
    read SECRET_VAULT_PLUGIN_TOKEN
    fi
fi

if [[ -z "${SECRET_VAULT_CONFIG_ROLE}" ]]; then
echo "Enter organization vault bamboo role as <roleid>:<secretid> - "
read SECRET_VAULT_CONFIG_ROLE
fi

if [[ -z "${SECRET_VAULT_PUB_ROLE}" ]]; then
echo "Enter organization vault config pub role as <roleid>:<secretid> - "
read SECRET_VAULT_PUB_ROLE
fi

if [[ -z "${KUBE_PATH}" ]]; then
echo "Enter organization kube config path: "
read KUBE_PATH
fi

VAULT_TOKEN=$AGENT_VAULT_TOKEN
VAULT_API_ADDR=$SECRET_VAULT_ADDR
VAULT_ADDR=$AGENT_VAULT_ADDR
SECRET_VAULT_ENV_TOKEN=$VAULT_ENV_TOKEN

export VAULT_ADDR
export VAULT_TOKEN
export VAULT_API_ADDR
export SECRET_VAULT_ENV_TOKEN
#===============================================================================
#carrier undeploy
#===============================================================================

echo "Disable old carrier secrets"
vault secrets disable vaultcarrier/
vault secrets list | grep trc-vault-carrier-plugin$PROD_EXT
existingplugin=$?
if [ $existingplugin -eq 0 ]; then
    echo "Carrier plugin still mounted elsewhere.  Manual intervention required to clean up before registration can proceed."
    exit 1
else
    echo "All mounts cleared.  Continuing..."
fi
echo "Unregister old carrier plugin"
vault plugin deregister trc-vault-carrier-plugin$PROD_EXT

echo "Registering Carrier"
echo trc-vault-carrier-plugin$PROD_EXT

#===============================================================================
#carrier deploy
#===============================================================================

vault plugin register \
          -command=trc-vault-carrier-plugin$PROD_EXT \
          -sha256=$( cat target/trc-vault-carrier-plugin$PROD_EXT.sha256 ) \
          -args=`backendUUID=567` \
          trc-vault-carrier-plugin$PROD_EXT
echo "Enabling Carrier secret engine"
vault secrets enable \
          -path=vaultcarrier \
          -plugin-name=trc-vault-carrier-plugin$PROD_EXT \
          -description="Tierceron Vault Carrier Plugin" \
          plugin

#===============================================================================
#refresh carrier token
#===============================================================================
echo "Refreshing carrier..."
sleep 12

export TRC_KUBE_CONFIG=`cat $KUBE_PATH | base64 --wrap=0`
VAULT_API_ADDR=VAULT_ADDR
echo $VAULT_ADDR

if [ -z "${PROD_EXT}" ]; then
    if [[ ! -z "${SECRET_VAULT_DEV_PLUGIN_TOKEN}" ]]; then
        echo "Refreshing carrier dev..."
        vault write vaultcarrier/dev token=$VAULT_ENV_TOKEN vaddress=$AGENT_VAULT_ADDR caddress=$SECRET_VAULT_ADDR ctoken=$SECRET_VAULT_DEV_PLUGIN_TOKEN pubrole=$SECRET_VAULT_PUB_ROLE configrole=$SECRET_VAULT_CONFIG_ROLE kubeconfig=$TRC_KUBE_CONFIG
    fi

    if [[ ! -z "${SECRET_VAULT_QA_PLUGIN_TOKEN}" ]]; then
        echo "Refreshing carrier QA..."
        vault write vaultcarrier/QA token=$VAULT_ENV_TOKEN vaddress=$AGENT_VAULT_ADDR caddress=$SECRET_VAULT_ADDR ctoken=$SECRET_VAULT_QA_PLUGIN_TOKEN pubrole=$SECRET_VAULT_PUB_ROLE configrole=$SECRET_VAULT_CONFIG_ROLE kubeconfig=$TRC_KUBE_CONFIG
    fi
else
    vault write vaultcarrier/$VAULT_ENV token=$VAULT_ENV_TOKEN vaddress=$AGENT_VAULT_ADDR caddress=$SECRET_VAULT_ADDR ctoken=$SECRET_VAULT_PLUGIN_TOKEN pubrole=$SECRET_VAULT_PUB_ROLE configrole=$SECRET_VAULT_CONFIG_ROLE kubeconfig=$TRC_KUBE_CONFIG
fi

echo "Deployment and refresh of carrier successful"

#================================================================
#trcsh deployment
#================================================================
echo "Deploying trcsh agent"
TRC_PLUGIN_NAME="trcsh"

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
    if [[ -z "${SECRET_VAULT_ENV_TOKEN}" ]]; then
       SECRET_VAULT_ENV_TOKEN=$SECRET_VAULT_DEV_PLUGIN_TOKEN
    fi
fi

echo "Certifying agent deployment tool plugin..."

trcplgtool -env=$VAULT_ENV -certify -addr=$SECRET_VAULT_ADDR -token=$SECRET_VAULT_ENV_TOKEN -pluginName=$TRC_PLUGIN_NAME -sha256=target/$TRC_PLUGIN_NAME -pluginType=agent
        certifystatus=$?
    if [ $certifystatus -eq 0 ]; then
        echo "No certification problems encountered."
    else
        echo "Unexpected certification error."
    fi

echo "Deployed trcsh agent successfully"

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
