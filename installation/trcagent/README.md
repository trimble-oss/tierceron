# Introduction 
You have found the installation folder for agent templates and secrets.

# Prerequisites
This assumes the existence of a vault with tokens.  You'll need a root or unrestricted token to initialize data
from here on out.  If you're a security purist, you'll already have deleted the root token at this point and
will just operate with the unrestricted dev token for the steps below.

# Generating empty seed files
trcpub -env=dev -token=$TRC_ROOT_TOKEN -addr=https://<vaulthost:vaultport>
trcx -env=dev -novault

# Edit seed files
At this point you want to edit all seed variables in preparation for publish.

# Publish initial trcdb seed data
trcinit -env=dev -token=$TRC_ROOT_TOKEN -addr=https://<vaulthost:vaultport>
