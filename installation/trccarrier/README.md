# Introduction 
You have found the installation folder for trc carrier.  This is a trusted vault
plugin utilized in the tierceron secure deployment services.  Carrier, working
in tandem with trcsh will interact with a docker registry and either virtual machines or a kubernetes cluster in order to securely deploy services.

# Prerequisites
This assumes the existence of a vault with tokens.  You also must have installed the build dependencies under [GETTING_STARTED.MD](../../GETTING_STARTED.MD#command-line-building-via-makefile). You'll need a root and unrestricted token install the carrier.

# Build carrier deployment scripts
trcconfig
chmod 700 deploy/*.sh

# Azure container registry configuration setup
```
trcx -env=dev -token=$VAULT_TOKEN -addr=$VAULT_ADDR -restricted=PluginTool -serviceFilter=config -indexFilter=config -novault
```

... after making edits to the generated seed file (all values can be TODO for local), init it.

```
trcinit -env=dev -token=$VAULT_TOKEN -addr=$VAULT_ADDR -restricted=PluginTool
```

# Building the carrier
cd ../../atrium
make certify devplugincarrier

# Deploy the carrier
trcplgtool -env=dev -certify -addr=$VAULT_ADDR -token=$VAULT_TOKEN -pluginName=trc-vault-carrier-plugin -sha256=target/trc-vault-carrier-plugin -pluginType=agent

sudo cp target/trc-vault-carrier-plugin /usr/local/vault/plugins
sudo setcap cap_ipc_lock=+ep /usr/local/vault/plugins/trc-vault-carrier-plugin

./deploycarrier.sh [Deploy Carrier](atrium/plugin/deploy)

# Trcsh integration (optional)
To bring carrier fully online, you'll also have to install trcsh as a plugin.  Trcsh only runs as a limited user
called azuredeploy.

sudo adduser --disabled-password --system --shell /bin/bash --group --home /home/azuredeploy azuredeploy
sudo mkdir -p /home/azuredeploy/bin
sudo chmod 1750 /home/azuredeploy/bin
sudo chown root:azuredeploy /home/azuredeploy/bin

./deploy.sh


./refreshcarriertoken.sh
