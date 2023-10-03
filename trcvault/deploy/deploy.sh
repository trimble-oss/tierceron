#!/bin/bash

echo "Enter plugin name: "
read TRC_PLUGIN_NAME

if [ "$TRC_PLUGIN_NAME" = 'trc-vault-carrier-plugin' ] ; then
    echo "Use deploycarrier to deploy carrier."
    exit 1
fi

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

echo "Enter agent vault host base url: "
read VAULT_ADDR

echo "Enter vault host name: "
read VAULT_HOSTNAME

if [[ -z "${SECRET_VAULT_ADDR}" ]]; then
echo "Enter organization vault host base url including port: "
read SECRET_VAULT_ADDR
fi

if [[ -z "${SECRET_VAULT_ENV_TOKEN}" ]]; then
echo "Enter organization vault unrestricted environment token with write permissions: "
read SECRET_VAULT_ENV_TOKEN
fi

if [[ -z "${SECRET_VAULT_PLUGIN_TOKEN}" ]]; then
echo "Enter organization vault plugin token for certification: "
read SECRET_VAULT_PLUGIN_TOKEN
fi

echo "Enter agent root token: "
read VAULT_TOKEN

echo "Enter agent environment: "
read VAULT_ENV

echo "Is this plugin an agent deployment tool (Y or N): "
read VAULT_AGENT

if [ "$VAULT_AGENT" = 'Y' ] || [ "$VAULT_AGENT" = 'y' ]; then
    PRE_CERTIFY="Y"
else
    echo "Enter trc plugin runtime environment token with write permissions unrestricted: "
    read VAULT_ENV_TOKEN

    if [ "$VAULT_PLUGIN_DIR" ]
    then
        echo "Deploying using local vault strategy."
        PRE_CERTIFY="N"
    else
        echo "Precertify plugin (Y or N): "
        read PRE_CERTIFY
    fi
fi


if [ "$PRE_CERTIFY" = "Y" ] || [ "$PRE_CERTIFY" = "yes" ] || [ "$PRE_CERTIFY" = "y" ]; then
    if [ "$VAULT_AGENT" = 'Y' ] || [ "$VAULT_AGENT" = 'y' ]; then 
        echo "Deploying agent deploy tool"
        trcplgtool -env=$VAULT_ENV -certify -addr=$SECRET_VAULT_ADDR -token=$SECRET_VAULT_ENV_TOKEN -pluginName=$TRC_PLUGIN_NAME -sha256=$(cat target/$TRC_PLUGIN_NAME.sha256) -pluginType=agent -hostName=$VAULT_HOSTNAME
        certifystatus=$?
        if [ $certifystatus -eq 0 ]; then       
           echo "No problems encountered."
           exit $certifystatus
        else
           echo "Unexpected certifyication errorerror."
           exit $certifystatus
        fi
    else
        trcplgtool -env=$VAULT_ENV -certify -addr=$SECRET_VAULT_ADDR -token=$SECRET_VAULT_ENV_TOKEN -pluginName=$TRC_PLUGIN_NAME -sha256=$(cat target/$TRC_PLUGIN_NAME.sha256) -hostName=$VAULT_HOSTNAME
    fi
fi

if [ "$VAULT_AGENT" = 'Y' ] || [ "$VAULT_AGENT" = 'y' ]; then
    exit 0
fi

if [ "$VAULT_PLUGIN_DIR" ]
then
    echo "Local plugin registration skipping certification."
else
    echo "Checking plugin deploy status."
    echo "Certifying plugin for env $VAULT_ENV."
    trcplgtool -env=$VAULT_ENV -checkDeployed -addr=$SECRET_VAULT_ADDR -token=$SECRET_VAULT_ENV_TOKEN -pluginName=$TRC_PLUGIN_NAME -sha256=$(cat target/$TRC_PLUGIN_NAME.sha256) -hostName=$VAULT_HOSTNAME
    status=$?
    echo "Plugin certified with result $status."

    if [ $status -eq 0 ]; then       
    echo "This version of the plugin has already been deployed - enabling for environment $VAULT_ENV."
    vault write $TRC_PLUGIN_NAME/$VAULT_ENV token=$VAULT_ENV_TOKEN vaddress=$VAULT_ADDR
    exit $status
    elif [ $status -eq 1 ]; then
    echo "Existing plugin does not match repository plugin - cannot continue."
    exit $status
    elif [ $status -eq 2 ]; then
    echo "This version of the plugin has not been deployed for environment $VAULT_ENV.  Beginning installation process."
    else
    echo "Unexpected error response."
    exit $status
    fi
fi
echo "Uninstalling existing plugin."

VAULT_API_ADDR=VAULT_ADDR
export VAULT_ADDR
export VAULT_TOKEN
export VAULT_API_ADDR

echo "Disable and unregister old plugin."
export VAULT_CLIENT_TIMEOUT=300s
# Migrate to carrier maybe?
vault secrets disable $TRC_PLUGIN_NAME/

vault secrets list | grep $TRC_PLUGIN_NAME
existingplugin=$?
if [ $existingplugin -eq 0 ]; then       
    echo "Plugin still mounted unexpectedly.  Manual intervention required to clean up before registration can proceed."
    exit 1
else
    echo "All mounts cleared.  Continuing..."
fi
vault plugin deregister $TRC_PLUGIN_NAME

sleep 12

if [ "$VAULT_PLUGIN_DIR" ]
then
    chmod 700 target/$TRC_PLUGIN_NAME
    cp target/$TRC_PLUGIN_NAME $VAULT_PLUGIN_DIR
    SHAVAL=$( cat target/$TRC_PLUGIN_NAME.sha256 )
    sudo setcap cap_ipc_lock=+ep $VAULT_PLUGIN_DIR/$TRC_PLUGIN_NAME
else
    # Just writing to vault will trigger a copy...
    # First we set Copied to false...
    # This should also trigger the copy process...
    # It should return sha256 of copied plugin on success.
    SHA256BUNDLE=$(vault write vaultcarrier/$VAULT_ENV plugin=$TRC_PLUGIN_NAME token=$VAULT_ENV_TOKEN vaddress=$VAULT_ADDR )
    SHAVAL=$(echo $SHA256BUNDLE | awk '{print $6}')

    if [ "$SHAVAL" = "Failure" ]; then
        echo "Failed to obtain sha256 for indicated plugin.   Refusing to continue."
        exit -1
    fi
fi

if [ "$SHAVAL" != "$FILESHAVAL" ]; then
    echo "Carrier failed to deploy plugin.  Please try again."
    exit -1
fi

PROD_EXT=""
for x in "staging" "prod"; do
    if [ $x = $VAULT_ENV ]; then
       PROD_EXT="-prod"
    fi
done

echo "Registering new plugin."
vault plugin register \
        -command=$TRC_PLUGIN_NAME$PROD_EXT \
        -sha256=$(echo $SHAVAL) \
        -args=`backendUUID=789` \
        $TRC_PLUGIN_NAME
echo "Enabling new plugin."

vault secrets enable \
        -path=$TRC_PLUGIN_NAME \
        -plugin-name=$TRC_PLUGIN_NAME \
        -description="Tierceron Vault Plugin" \
        plugin

#Activates/starts the deployed plugin.
# Note: plugin should update deployed flag for itself.
vault write $TRC_PLUGIN_NAME/$VAULT_ENV token=$VAULT_ENV_TOKEN vaddress=$VAULT_ADDR caddress=$SECRET_VAULT_ADDR ctoken=$SECRET_VAULT_PLUGIN_TOKEN

vault write vaultcarrier/$VAULT_ENV plugin=$TRC_PLUGIN_NAME