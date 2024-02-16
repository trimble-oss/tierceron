# Introduction 
You have found the installation folder for trcdb templates and secrets.

# Prerequisites
This assumes the existence of a vault with tokens.  You'll need a root or unrestricted token to initialize data
from here on out.  If you're a security purist, you'll already have deleted the root token at this point and
will just operate with the unrestricted dev token for the steps below.

# Generating empty seed files
trcpub -env=dev -token=$TRC_ROOT_TOKEN -addr=https://<vaulthost:vaultport>
trcx -env=dev -novault
trcx -env=dev -certs -novault

# Edit seed files and provide certificates.
At this point you want to edit all seed variables in preparation for publish.
After running trcx -certs, a certs folder will appear under trc_seeds with placeholder empty certificate files.
You'll want to replace these placeholder files with the real thing.

# Publish initial trcdb seed data
trcinit -env=dev -token=$TRC_ROOT_TOKEN -addr=https://<vaulthost:vaultport> -indexed=TrcVault
trcinit -env=dev -certs -token=$TRC_ROOT_TOKEN -addr=https://<vaulthost:vaultport>

# Agent installation
trcx -env=dev -token=$VAULT_TOKEN -restricted=TrcshAgent -serviceFilter=config -indexFilter=config -addr=$VAULT_ADDR -novault

trcinit -env=dev -token=$VAULT_TOKEN -addr=$VAULT_ADDR -restricted=TrcshAgent
