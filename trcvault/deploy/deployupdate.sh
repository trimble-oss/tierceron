#!/bin/bash

echo "Enter plugin name: "
read TRC_PLUGIN_NAME

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

echo "Enter vault host base url: "
read VAULT_ADDR

echo "Enter root token: "
read VAULT_TOKEN

echo "Enter environment: "
read VAULT_ENV

echo "Enter trc plugin runtime environment token with write permissions unrestricted: "
read VAULT_ENV_TOKEN

echo "Enter carrier deployment runtime token pluginEnv: "
read VAULT_CARRIER_DEPLOY_TOKEN

if [ "$VAULT_PLUGIN_DIR" ]
then
    echo "Deploying using local vault strategy."
    PRE_CERTIFY="N"
else
    echo "Precertify plugin: "
    read PRE_CERTIFY
fi

if [ "$VAULT_ENV" = "prod" ] || [ "$VAULT_ENV" = "staging" ]; then
    if [ "$PRE_CERTIFY" = "Y" ] || [ "$PRE_CERTIFY" = "yes" ]; then
    trcplgtool -env=$VAULT_ENV -certify -addr=$VAULT_ADDR -token=$VAULT_TOKEN -insecure -pluginName=$TRC_PLUGIN_NAME-prod -sha256=$(cat target/$TRC_PLUGIN_NAME-prod.sha256)
    fi
else
    if [ "$PRE_CERTIFY" = "Y" ] || [ "$PRE_CERTIFY" = "yes" ]; then
    trcplgtool -env=$VAULT_ENV -certify -addr=$VAULT_ADDR -token=$VAULT_TOKEN -insecure -pluginName=$TRC_PLUGIN_NAME -sha256=$(cat target/$TRC_PLUGIN_NAME.sha256)
    fi
fi

if [ "$VAULT_PLUGIN_DIR" ]
then
    echo "Local plugin registration skipping certification."
else
    echo "Checking plugin deploy status."
    if [ "$VAULT_ENV" = "prod" ] || [ "$VAULT_ENV" = "staging" ]; then
    echo "Certifying plugin for env $VAULT_ENV."
    trcplgtool -env=$VAULT_ENV -checkDeployed -addr=$VAULT_ADDR -token=$VAULT_TOKEN -insecure -pluginName=$TRC_PLUGIN_NAME-prod -sha256=$(cat target/$TRC_PLUGIN_NAME-prod.sha256)
    status=$?
    echo "Plugin certified with result $status."
    else
    echo "Certifying plugin for env $VAULT_ENV."
    trcplgtool -env=$VAULT_ENV -checkDeployed -addr=$VAULT_ADDR -token=$VAULT_TOKEN -insecure -pluginName=$TRC_PLUGIN_NAME -sha256=$(cat target/$TRC_PLUGIN_NAME.sha256)
    status=$?
    echo "Plugin certified with result $status."
    fi 

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

VAULT_API_ADDR=VAULT_ADDR
export VAULT_ADDR
export VAULT_TOKEN
export VAULT_API_ADDR

if [ "$VAULT_ENV" = "prod" ] || [ "$VAULT_ENV" = "staging" ]; then
    # Just writing to vault will trigger the carrier plugin...
    # First we set Copied to false...
    # This should also trigger the copy process...
    # It should return sha256 of copied plugin on success.
    SHA256BUNDLE=$(vault write vaultcarrier/$VAULT_ENV token=$VAULT_CARRIER_DEPLOY_TOKEN plugin=$TRC_PLUGIN_NAME-prod vaddress=$VAULT_ADDR)
    SHAVAL=$(echo $SHA256BUNDLE | awk '{print $6}')
    
    if [ "$SHAVAL" != "$FILESHAVAL" ]; then
    echo "Carrier failed to deploy plugin.  Please try again."
    exit -1
    fi

    if [ "$SHAVAL" = "" ]; then
    echo "Failed to obtain sha256 for indicated plugin.   Refusing to continue."
    exit -1
    fi

    echo "Registering new plugin."
    vault plugin register \
            -command=$TRC_PLUGIN_NAME-prod \
            -sha256=$(echo $SHAVAL) \
            -args=`backendUUID=789` \
            $TRC_PLUGIN_NAME-prod
    echo "Enabling new plugin."
    vault plugin reload \
            -plugin $TRC_PLUGIN_NAME-prod
else
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
        SHA256BUNDLE=$(vault write vaultcarrier/$VAULT_ENV token=$VAULT_CARRIER_DEPLOY_TOKEN plugin=$TRC_PLUGIN_NAME vaddress=$VAULT_ADDR)
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

    echo "Registering new plugin."
    vault plugin register \
            -command=$TRC_PLUGIN_NAME \
            -sha256=$(echo $SHAVAL) \
            -args=`backendUUID=789` \
            $TRC_PLUGIN_NAME
    echo "Enabling new plugin."
    vault plugin reload \
            -plugin $TRC_PLUGIN_NAME
fi


#Activates/starts the deployed plugin.
# Note: plugin should update deployed flag for itself.
vault write $TRC_PLUGIN_NAME/$VAULT_ENV token=$VAULT_ENV_TOKEN vaddress=$VAULT_ADDR

