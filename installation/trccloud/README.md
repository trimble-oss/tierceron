# Introduction 
The installation folder for trccloud.

# Prerequisites
This assumes you have access to a cloud infrastructure (az or aws cli) and have performed the
initial setup.  You also need to have installed the terraform client.

wanted -- gcloud client terraform scripts?

# Build initial cloud infrastructure
```
cd terraform/azure/cloudboot
```
- or -  

cd terraform/aws -- untested.  Would be nice if someone would test and confirm this either works or doesn't.  

# Generating empty seed files
```
trcx -env=dev -novault
```

# Edit seed files and provide certificates.
At this point you want to edit all seed variables in preparation for publish.  
After running trcx -certs, a certs folder will appear under trc_seeds with placeholder empty certificate files.  
You'll want to replace these placeholder files with the real thing.  

# Generate terraform script files.
```
mkdir trc_seeds
trcx -env=dev -novault
trcconfig -env=dev -novault
```

# Generate your infrastructure.
```
terraform init
terraform apply
```

# Optional: later, after initializing trcvault, you can perform this step: Publish terraform seed data to vault.
```
trcinit -env=dev -token=$VAULT_TOKEN -addr=https://<vaulthost:vaultport> -indexed=TrcVault
```

```
trcinit -env=dev -certs -token=$VAULT_TOKEN -addr=https://<vaulthost:vaultport>
```

