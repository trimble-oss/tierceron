# Introduction 
You have found the installation folder for trcsh.  This is a trusted vault
plugin utilized in the tierceron secure deployment services.  Carrier, working
in tandem with trcsh will interact with a docker registry and either virtual machines 
or a kubernetes cluster in order to securely deploy services.

# Prerequisites
This assumes the existence of a vault with tokens.  You also must have installed the build dependencies under [GETTING_STARTED.MD](../../GETTING_STARTED.MD#command-line-building-via-makefile). You'll need a root and unrestricted token install the carrier.  You should also have already installed trccarrier and set up some kind of container
registry.

# Trcsh server deployer integration
To bring carrier fully online, you'll also have to install trcsh as a plugin.  Trcsh only runs as a restricted user called azuredeploy so you'll need to make it now.

```
sudo adduser --disabled-password --system --shell /bin/bash --group --home /home/azuredeploy azuredeploy
sudo mkdir -p /home/azuredeploy/bin
sudo chmod 1750 /home/azuredeploy/bin
sudo chown root:azuredeploy /home/azuredeploy/bin
```

Deploy trcsh as a server now by running...

```
./deploy/deploy.sh
```

# Feathering client and server configuration setup (optional)
If you want to support remote trcsh server deployments, you'll need to set up this this additional section.
This allows trcsh to run both as a client and a server to perform deployments.

```
trcx -env=dev -token=$VAULT_TOKEN -addr=$VAULT_ADDR -restricted=TrcshAgent -serviceFilter=config -indexFilter=config -novault
```

... after making edits to the generated seed file (all values can be TODO for local), init it.

```
trcinit -env=dev -token=$VAULT_TOKEN -addr=$VAULT_ADDR -restricted=TrcshAgent
```



# Trcsh client integration
To bring deployments fully online, you'll need to install the trcsh script executable on each virtual
machine you'd like to perform deployments under.

```
sudo adduser --disabled-password --system --shell /bin/bash --group --home /home/trcshd trcshd
sudo mkdir -p /home/trcshd/bin
sudo chmod 1750 /home/trcshd/bin
sudo chown root:trcshd /home/trcshd/bin

cp trcsh /home/trcshd/bin

```

TODO: create install script to run trcsh as a service on linux... or windows...