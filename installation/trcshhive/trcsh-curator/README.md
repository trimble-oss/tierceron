# Introduction 
You have found the installation folder for trc curator.  This is a trusted vault
plugin utilized in the tierceron secure deployment services.  Carrier, working
in tandem with trcsh will interact with a docker registry and either virtual machines or a kubernetes cluster in order to securely deploy services.

# Prerequisites
This assumes the existence of a vault with tokens.  You also must have installed the build dependencies under [GETTING_STARTED.MD](../../../GETTING_STARTED.MD#command-line-building-via-makefile). You'll need a root and unrestricted token install the curator.

# Container registry configuration setup
Trcsh utilizes cursor and vault managed secrets in order to access a container registry (running in AWS, Azure, or locally) to perform it's deployment responsibilities.  Set up the container configuration secrets with the following command.

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
If you want to support trcsh windows deployments and or trcsh kernel (for hive infrastructure),
See [installation/trcsh/README.md](../trcsh/README.md)


# Building the curator
```
pushd .
cd ../../../atrium
make certify cursor
popd
```

# Generate deployment scripts
```
trcpub -env=dev -token=$VAULT_TOKEN -addr=https://<vaulthost:vaultport>
trcconfig -env=dev -startDir=trc_templates/TrcVault/Carrier
chmod 700 deploy/*.sh
```

# Deploy the curator
```
cd deploy
trcplgtool -env=dev -certify -addr=$VAULT_ADDR -token=$VAULT_TOKEN -pluginName=trcsh-curator -sha256=target/trcsh-curator -pluginType=agent

sudo cp target/trcsh-curator /usr/local/vault/plugins
sudo setcap cap_ipc_lock=+ep /usr/local/vault/plugins/trcsh-curator

./deploycurator.sh
```

# Trcsh server deployer integration (optional)
To bring curator fully online, you'll also have to install trcsh as a plugin.  Trcsh only runs as a restricted user called azuredeploy so you'll need to make it now.

```
sudo adduser --disabled-password --system --shell /bin/bash --group --home /home/azuredeploy azuredeploy
sudo mkdir -p /home/azuredeploy/bin
sudo chmod 1750 /home/azuredeploy/bin
sudo chown root:azuredeploy /home/azuredeploy/bin

./deploy.sh

./refreshcuratortoken.sh
```

# Docker registry setup (local development only)
Trcsh works based on a registry to perform deployments.  For personal use, you can easily install a local docker registry.  For organizations, a cloud implementation will give you stability, redundancy, and easier accessiblity.  But for development, a local registry is perfect!  Start by installing docker.

https://docs.docker.com/engine/install/debian/#install-using-the-repository

## Create docker swarm
You'll want a docker swarm to more easily set up and manage a docker registry service.

```
docker swarm init
```

This command will output a couple commands to run.  Don't worry about saving the tokens as you can always get one later...

```
Skip these commands...
docker swarm join-token worker
docker swarm join --token <swarm token> <ip>:<port>
docker swarm join-token manager
```

The following command(s) will set up the actual registry and test it was
properly set up.

```
docker service create --name <yourregistryname> --publish published=<yourregistryport>,target=5000 registry:2

See service running
docker service ls

Test accessibility
curl http://<localip>:<yourregistryport>/v2/

# Add any users to the docker group
usermod -a -G docker <username>

# Add an image to docker repository
docker build -t <imagename> .

# List images
docker images

# Delete an image
docker rmi <image id>

```

# Trcsh client integration
