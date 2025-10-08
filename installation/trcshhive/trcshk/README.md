# Introduction 
You have found the installation folder for trcsh kernel templates and secrets.  These secrets are required for running trcsh.exe as a windows service or trcshk as a linux daemon running in collaboration with the curator to perform deployments on a different machine.

# Prerequisites
This assumes the existence of a vault with tokens.  You'll need a root or unrestricted token to initialize data from here on out.  If you're a security purist, you'll already have deleted the root token at this point and will just operate with the unrestricted dev token for the steps below.
Agent *requires* trcsh to be set up prior to use.  Trcsh running on the server coordinates all activities within the agent.  In order to function, the additional feathering configurations must also be set up for each environment in which the agent operates.

# Agent installation
```
trcpub -env=dev -token=$VAULT_TOKEN -addr=https://<vaulthost:vaultport>
trcx -env=dev -token=$VAULT_TOKEN -restricted=TrcshCursorAW -serviceFilter=config -indexFilter=config -addr=$VAULT_ADDR -novault

```

# Edit seed files
At this point you want to edit all seed variables in preparation for publish.

```
trcinit -env=dev -token=$VAULT_TOKEN -addr=$VAULT_ADDR -restricted=TrcshCursorAW
```


# Trcsh client integration
To bring deployments fully online, you'll need to install the trcsh script executable on each virtual machine you'd like to perform deployments under.  The following creates a dedicated trcshk user for performing deployments.  A trcshk daemon or service will run and wait for deployment commands initiated by the curator.

```
sudo adduser --disabled-password --system --shell /bin/bash --group --home /home/trcshk trcshk
sudo mkdir -p /home/trcshk/bin
sudo chmod 1750 /home/trcshk/bin
sudo chown root:trcshk /home/trcshk/bin

cp ../trcsh-curator/deploy/target/trcsh /home/trcshk/bin
sudo chown root:trcshk /home/trcshk/bin/trcsh
sudo setcap cap_ipc_lock=+ep /home/trcshk/bin/trcsh

```


# Agent machine setup
In addition to setting up trcsh to run as a remote agent, you'll need to specify a list of one or more deployments, a supported environment, and optionally, a script deployment path.  Each agent presently is only capable of referencing a single deployment script path.  In order to support multiple deployments in a single project/service, you need to create separate project/service template sets each including their own deploy/deploy.trc.tmpl templates.


In order to remote deploy the script, trcsh running in the context of an agent in the Tierceron agent pool, trcsh will execute the trcplgtool with a -agentdeploy command.  This command will trigger any listening agents for the specified environment to wake up and execute the deployment script stored in the vault.  Each running agent *must* have a dedicated environment (env or env-x where env is one of dev, QA, RQA, etc… and x is a number from 1…max(int))


On a target machine, execute setup.cmd for a Windows environment, or install.sh for a linux environment.  These scripts will install a service that will run with the following environment variables.

```
chmod 700 install.sh
./install.sh

```

Required environment variables:
DEPLOYMENTS=a,b,c
VAULT_ADDR=
AGENT_TOKEN=
AGENT_ENV= (used if -env is not specified in execution of trcsh from command line)

Supported trcsh cmd arguments:
-env={env}

When trcsh server runs and triggers using the trcplgtool with -agentdeploy, the script <trcprojectservice>/deploy/deploy.trc.tmpl will get pulled, populated and executed on the remote machine.
