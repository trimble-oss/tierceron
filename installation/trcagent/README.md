# Introduction 
You have found the installation folder for agent templates and secrets.  These secrets are required for running trcsh as a windows or linux daemon running in collaboration with the carrier to perform deployments on a different machine.

# Prerequisites
This assumes the existence of a vault with tokens.  You'll need a root or unrestricted token to initialize data
from here on out.  If you're a security purist, you'll already have deleted the root token at this point and
will just operate with the unrestricted dev token for the steps below.

# Agent installation
```
trcpub -env=dev -token=$TRC_ROOT_TOKEN -addr=https://<vaulthost:vaultport>
trcx -env=dev -token=$VAULT_TOKEN -restricted=TrcshAgent -serviceFilter=config -indexFilter=config -addr=$VAULT_ADDR -novault

```

# Edit seed files
At this point you want to edit all seed variables in preparation for publish.

```
trcinit -env=dev -token=$VAULT_TOKEN -addr=$VAULT_ADDR -restricted=TrcshAgent
```

# Agent machine setup
In order to set up trcsh to run as a remote agent, you'll need to specify a list of one or more deployments, a supported environment, and optionally, a script deployment path.  Each agent presently is only capable of referencing a single deployment script path.  In order to support multiple deployments in a single project/service, you need to create separate project/service template sets each including their own deploy/deploy.trc.tmpl templates.


In order to remote deploy the script, trcsh running in the context of an agent in the Tierceron agent pool, trcsh will execute the trcplgtool with a -agentdeploy command.  This command will trigger any listening agents for the specified environment to wake up and execute the deployment script stored in the vault.  Each running agent *must* have a dedicated environment (env or env-x where env is one of dev, QA, RQA, etc… and x is a number from 1…max(int))


On a target machine, the above variables are defined using the following:
Required environment variables:
DEPLOYMENTS=a,b,c
VAULT_ADDR=
AGENT_TOKEN=
AGENT_ENV= (used if -env is not specified in execution of trcsh from command line)

Supported trcsh cmd arguments:
-env={env}

When trcsh server runs and triggers using the trcplgtool with -agentdeploy, the script <trcprojectservice>/deploy/deploy.trc.tmpl will get pulled, populated and executed on the remote machine.
