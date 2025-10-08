# Introduction 
The installation folder for trcfssync.  This is if you want to install blob fusing using tierceron to manage secrets.

# Prerequisites
You *must* have all trc cmd line utilities installed as explained in GETTING_STARTED.md.  You must also have completed
either trclocal (or trccloud) and trcvault.

# Build initial cloud infrastructure
TODO

# Generating empty seed files
```
mkdir trc_seeds
trcx -env=dev -novault
```

# Edit seed files and provide certificates.
At this point you want to edit all seed variables in preparation for publish.

Fill in seed variables in super-secrets section of trc_seeds/dev/dev_seed.yml

# Optional: later, after initializing trcvault, you can perform this step: Publish terraform seed data to vault.
```
trcinit -env=dev -token=$VAULT_TOKEN -addr=https://<vaulthost:vaultport> -indexed=TrcVault
```

# Create cert placeholder files
TODO Needed?

# Generate vault properties configuration
```
trcconfig -env=dev -novault
```
sudo cp configs to configs...

# Start vault as a service.
TODO...

