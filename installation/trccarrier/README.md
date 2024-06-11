# Introduction 
You have found the installation folder for trc carrier.  This is a trusted vault
plugin utilized in the tierceron secure deployment services.  Carrier, working
in tandem with trcsh will interact with a docker registry and either virtual machines or a kubernetes cluster in order to securely deploy services.

# Prerequisites
This assumes the existence of a vault with tokens.  You also must have installed the build dependencies under [GETTING_STARTED.MD](../../GETTING_STARTED.MD#command-line-building-via-makefile). You'll need a root and unrestricted token install the carrier.

# Build carrier deployment scripts
trcconfig
chmod 700 deploy/*.sh

# Container registry configuration setup
Trcsh utilizes trccarrier and vault managed secrets in order to access a container registry (running in AWS, Azure, or locally) to perform it's deployment responsibilities.  Set up the container configuration secrets with the following command.

Choose one of the 3 following to set up a container registry flavor of your choice...

Local Container Registry Setup (for local development)
```
trcx -env=dev -token=$VAULT_TOKEN -addr=$VAULT_ADDR -restricted=PluginTool -serviceFilter=local-config -indexFilter=local-config -novault
```

Azure Container Registry Setup
```
trcx -env=dev -token=$VAULT_TOKEN -addr=$VAULT_ADDR -restricted=PluginTool -serviceFilter=config -indexFilter=config -novault
```

AWS Container Registry Setup
```
trcx -env=dev -token=$VAULT_TOKEN -addr=$VAULT_ADDR -restricted=PluginTool -serviceFilter=aws-config -indexFilter=aws-config -novault
```

... after making edits to the generated seed file (all values can be TODO for local), init it.

```
trcinit -env=dev -token=$VAULT_TOKEN -addr=$VAULT_ADDR -restricted=PluginTool
```

# Feathering configuration setup (optional)
If you want to support trcsh windows deployments, you'll need to set up this section.

```
trcx -env=dev -token=$VAULT_TOKEN -addr=$VAULT_ADDR -restricted=TrcshAgent -serviceFilter=config -indexFilter=config -novault
```

... after making edits to the generated seed file (all values can be TODO for local), init it.

```
trcinit -env=dev -token=$VAULT_TOKEN -addr=$VAULT_ADDR -restricted=TrcshAgent
```


# Building the carrier
```
pushd .
cd ../../atrium
make certify devplugincarrier
popd
```

# Generate deployment scripts
```
trcconfig -env=dev -startDir=trc_templates/TrcVault/Carrier -novault
```

# Deploy the carrier
```
cd deploy
trcplgtool -env=dev -certify -addr=$VAULT_ADDR -token=$VAULT_TOKEN -pluginName=trc-vault-carrier-plugin -sha256=target/trc-vault-carrier-plugin -pluginType=agent

sudo cp target/trc-vault-carrier-plugin /usr/local/vault/plugins
sudo setcap cap_ipc_lock=+ep /usr/local/vault/plugins/trc-vault-carrier-plugin

./deploycarrier.sh
```

# Trcsh server deployer integration (optional)
To bring carrier fully online, you'll also have to install trcsh as a plugin.  Trcsh only runs as a restricted user called azuredeploy so you'll need to make it now.

```
sudo adduser --disabled-password --system --shell /bin/bash --group --home /home/azuredeploy azuredeploy
sudo mkdir -p /home/azuredeploy/bin
sudo chmod 1750 /home/azuredeploy/bin
sudo chown root:azuredeploy /home/azuredeploy/bin

./deploy.sh

./refreshcarriertoken.sh
```

# Docker registry setup (local development only)
Trcsh works based on a registry to perform deployments.  For personal use, you can easily
install a local docker registry.  Start by installing docker

https://docs.docker.com/desktop/install/debian/


# Trcsh client integration
