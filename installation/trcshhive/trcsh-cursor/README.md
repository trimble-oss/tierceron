# Introduction 
You have found the installation folder for trcsh.  This is a trusted tierceron secure shell utilized in the tierceron secure deployment services.  Carrier, working in tandem with trcsh will interact with a docker registry and either virtual machines or a kubernetes cluster in order to securely deploy services.

# Prerequisites
This assumes the existence of a vault with tokens.  You also must have installed the build dependencies under [GETTING_STARTED.MD](../../../GETTING_STARTED.MD#command-line-building-via-makefile). You'll need a root and unrestricted token install the curator.  You should also have already installed trcsh-curator and set up some kind of container registry.

# Trcsh server deployer integration
To bring curator fully online, you'll also have to install trcsh as a plugin.  Trcsh only runs as a restricted user called azuredeploy so you'll need to make it now.

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

# Feathering client and server configuration setup (optional curator)
If you want to support remote trcsh server deployments, you'll need to set up this this additional section.
This allows trcsh to run both as a client and a server to perform deployments.

```
trcx -env=dev -token=$VAULT_TOKEN -addr=$VAULT_ADDR -restricted=TrcshCurator -serviceFilter=config -indexFilter=config -novault
```

... after making edits to the generated seed file (all values can be TODO for local), init it.  Hint, you can use pwgen to generate some good passwords.

```
trcpub -env=dev -token=$VAULT_TOKEN -addr=$VAULT_ADDR
trcinit -env=dev -token=$VAULT_TOKEN -addr=$VAULT_ADDR -restricted=TrcshCurator
```

# Feathering configuration setup for kernel messaging support (optional)
If you want to support the trcsh kernel hive infrastructure, you'll need to install Trcshm

```
trcx -env=dev -token=$VAULT_TOKEN -addr=$VAULT_ADDR -restricted=TrcshCursorAW -serviceFilter=config -indexFilter=config -novault

trcx -env=dev -token=$VAULT_TOKEN -addr=$VAULT_ADDR -restricted=TrcshCursorK -serviceFilter=config -indexFilter=config -novault

```

... after making edits to the generated seed file (all values can be TODO for local), init it.  These must
be distinct from TrcshCursor for proper functioning.

```
trcinit -env=dev -token=$VAULT_TOKEN -addr=$VAULT_ADDR -restricted=TrcshCursorAW

trcinit -env=dev -token=$VAULT_TOKEN -addr=$VAULT_ADDR -restricted=TrcshCursorK

```


Install the trcshd service (Linux)
TODO: create install script to run trcsh as a service on linux... or windows...