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
