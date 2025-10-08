#!/bin/bash

echo "This script will install and certify a new hive including a trcshq, trcsh-cursor-x, and trcshk or trcsh.exe components."

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
echo "Enter organization vault host base url including port (hit enter same host as agent vault host): "
read SECRET_VAULT_ADDR
fi

if [[ "${SECRET_VAULT_ADDR}" == "" ]]; then
SECRET_VAULT_ADDR=$AGENT_VAULT_ADDR
elif [[ ! "${SECRET_VAULT_ADDR}" == "https://"* ]]; then
echo "Organization vault host must begin with https:// "
exit -1
else
echo "Organization vault host must begin with https:// "
fi



echo "Will this be a (aw - traditional aks and windows), (k - advanced aks hive kernel), or (b - both) cursor? (aw, k, or b): "
read CURSOR_TYPE_IN

CURSOR_TYPES=()

if [ "$CURSOR_TYPE_IN" = 'aw' ] || [ "$CURSOR_TYPE_IN" = 'k' ]; then
    CURSOR_TYPES+=("$CURSOR_TYPE_IN")
    echo "Preparing to install cursor type $CURSOR_TYPE_IN..."
elif [ "$CURSOR_TYPE_IN" = 'b' ]; then
    CURSOR_TYPES+=("aw")
    CURSOR_TYPES+=("k")
else
    echo "Unspported cursor type $CURSOR_TYPE_IN."
    exit 1
fi

if [[ -z "${VAULT_ENV}" ]]; then
    echo "Enter organization/agent environment: "
    read VAULT_ENV
fi


echo "Is this a prod environment? (Y or N): "
read PROD_ENV

if [ "$PROD_ENV" = 'Y' ] || [ "$PROD_ENV" = 'y' ]; then
PROD_EXT=""
    for x in "staging" "prod"; do
        if [ $x = $VAULT_ENV ]; then
           PROD_EXT="-prod"
        fi
    done

    if [[ -z "${SECRET_VAULT_TOKEN}" ]]; then
        echo "Enter organization root token: "
        read SECRET_VAULT_TOKEN
    fi
else
    SECRET_VAULT_TOKEN=$VAULT_TOKEN
fi

VAULT_TOKEN=$AGENT_VAULT_TOKEN
VAULT_API_ADDR=$SECRET_VAULT_ADDR
VAULT_ADDR=$AGENT_VAULT_ADDR

export VAULT_ADDR
export VAULT_TOKEN
export VAULT_API_ADDR

function curatorDeployHive () {
    CURSOR_TYPE=$1
    #===============================================================================
    #trcsh-cursor-$CURSOR_TYPE undeploy
    #===============================================================================

    echo "Disable old trcsh-cursor-$CURSOR_TYPE$PROD_EXT secrets"
    vault plugin deregister trcsh-cursor-$CURSOR_TYPE$PROD_EXT
    vault secrets disable trcsh-cursor-$CURSOR_TYPE$PROD_EXT/
    vault secrets list | grep trcsh-cursor-$CURSOR_TYPE$PROD_EXT
    existingplugin=$?
    if [ $existingplugin -eq 0 ]; then
        echo "trcsh-cursor-$CURSOR_TYPE$PROD_EXT plugin still mounted elsewhere.  Manual intervention required to clean up before registration can proceed."
        exit 1
    else
        echo "All mounts cleared.  Continuing..."
    fi
    echo "Unregister old trcsh-cursor-$CURSOR_TYPE$PROD_EXT plugin"
    vault plugin deregister trcsh-cursor-$CURSOR_TYPE$PROD_EXT

    echo "Registering trcsh-cursor-$CURSOR_TYPE$PROD_EXT"

    #===============================================================================
    #trcsh-cursor-$CURSOR_TYPE deploy
    #===============================================================================

    if [ -z "${PROD_EXT}" ]; then
        VAULT_ENV="dev"
    fi

    trcplgtool -env=$VAULT_ENV -certify -addr=$SECRET_VAULT_ADDR -token=$SECRET_VAULT_TOKEN -pluginName=trcsh-cursor-$CURSOR_TYPE$PROD_EXT -sha256=target/trcsh-cursor-$CURSOR_TYPE$PROD_EXT

    certifystatus=$?
    if [ $certifystatus -eq 0 ]; then
    echo "No certification problems encountered."
    else
    echo "Unexpected certification error."
    exit $certifystatus
    fi

    #================================================================
    #trcshq$CURSOR_TYPE deployment (the hive queen)
    #================================================================
    echo "Preparing trcshq$CURSOR_TYPE for certification"
    TRC_PLUGIN_NAME="trcshq$CURSOR_TYPE"

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
    fi

    echo "Certifying trcshq$CURSOR_TYPE"

    trcplgtool -env=$VAULT_ENV -certify -addr=$SECRET_VAULT_ADDR -token=$SECRET_VAULT_TOKEN -pluginName=$TRC_PLUGIN_NAME -sha256=target/$TRC_PLUGIN_NAME -pluginType=agent
            certifystatus=$?
        if [ $certifystatus -eq 0 ]; then
            echo "No certification problems encountered."
            echo "trcshq$CURSOR_TYPE successfully certified"
        else
            echo "Unexpected certification error."
            echo "trcshq$CURSOR_TYPE failed certification"
        fi
}

function certifyWorkers() {
    CURSOR_TYPE=$1

    if [ "$CURSOR_TYPE" = 'b' ] || [ "$CURSOR_TYPE" = 'aw' ]; then
        #================================================================
        #trcsh.exe deployment
        #================================================================
        echo "Certifying trcsh.exe worker"
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
            VAULT_ENV="servicepack"
            vault kv patch super-secrets/$VAULT_ENV/Index/TrcVault/trcplugin/$TRC_PLUGIN_NAME/Certify trcsha256=$FILESHAVAL
            VAULT_ENV="RQA"
            vault kv patch super-secrets/$VAULT_ENV/Index/TrcVault/trcplugin/$TRC_PLUGIN_NAME/Certify trcsha256=$FILESHAVAL
            echo "trcsh.exe certified and ready for manual deployment."
        else
            echo "Skipping $TRC_PLUGIN_NAME deployment in prod."
        fi

    fi
}

function certifyKernelWorker() {
    CURSOR_TYPE=$1

    if [ "$CURSOR_TYPE" = 'b' ] || [ "$CURSOR_TYPE" = 'k' ]; then
        #================================================================
        #trcshk deployment
        #================================================================
        echo "Certifying trcshk worker"
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

        echo "trcshk certified and ready for release pipeline deployment"
    fi
}

function registerCursors() {
    CURSOR_DEPLOY_TYPE=$1
    #================================================================
    #trcsh-cursor-$CURSOR_TYPE$PROD_EXT deployment (the cursor)
    #================================================================
    vault plugin register \
            -command=trcsh-cursor-$CURSOR_DEPLOY_TYPE$PROD_EXT \
            -sha256=$( cat target/trcsh-cursor-$CURSOR_DEPLOY_TYPE$PROD_EXT.sha256 ) \
            -args=`backendUUID=567` \
            trcsh-cursor-$CURSOR_DEPLOY_TYPE$PROD_EXT
    echo "Enabling trcsh-cursor-$CURSOR_DEPLOY_TYPE$PROD_EXT secret engine"
    vault secrets enable \
            -path=trcsh-cursor-$CURSOR_DEPLOY_TYPE$PROD_EXT \
            -plugin-name=trcsh-cursor-$CURSOR_DEPLOY_TYPE$PROD_EXT \
            -description="Tierceron Vault trcsh-cursor-$CURSOR_DEPLOY_TYPE$PROD_EXT Plugin" \
            plugin

    echo "Deployment and refresh of trcsh-cursor-$CURSOR_DEPLOY_TYPE$PROD_EXT successful"
}

# Deploy curator only for dev and staging
if [[ "$VAULT_ENV" == "dev" || "$VAULT_ENV" == "staging" ]]; then
    for cursorType in ${CURSOR_TYPES[@]}; do
        curatorDeployHive $cursorType
    done

    echo "Hive deployed successfully? trcshqx and trcsh-cursor-x must at least have deployed by this point Y/n: "
    read DEPLOYED_SUCCESS

    if [ "$DEPLOYED_SUCCESS" = 'N' ] || [ "$DEPLOYED_SUCCESS" = 'n' ]; then
        exit 1
    fi
fi

# Register cursors only for dev and staging
if [[ "$VAULT_ENV" == "dev" || "$VAULT_ENV" == "staging" ]]; then
    for cursorType in ${CURSOR_TYPES[@]}; do
        registerCursors $cursorType
    done
fi

for cursorType in ${CURSOR_TYPES[@]}; do
    # Certify Kernel
    certifyKernelWorker $cursorType
    # Certify Workers
    certifyWorkers $cursorType
done
