
## Let's get started
To set up tierceron secret and configuration management system you can do any of the following...

### Install command line tools
See [installation/GETTING_STARTED.md](installation/GETTING_STARTED.md)

### Setting up local vault (for new individuals or organizations without a vault set up yet)
See [installation/trclocal/README.md](installation/trclocal/README.md)

### Setting up a cloud hosted vault (skip if using local vault.  only for setting up in cloud -- needs cleanup)
See [installation/trccloud/terraform/azure/cloudboot/GETTING_STARTED.MD](installation/trccloud/terraform/azure/cloudboot/GETTING_STARTED.MD)

### Setting up default tokens and approles using example policy, role, and token configuration files (only needed to set up local or cloud instance of vault)
See [installation/trcvault/README.md](installation/trcvault/README.md)

### Setting up your very first hello world example application
See hello world section of [installation/trclocal/trchelloworld/README.md](installation/trclocal/trchelloworld/README.md)

### Installing collaborative deployment system (trc-vault-carrier and trcsh)
Run trcconfig under (installation/trccloud/terraform/azure/cloudboot)

deploycarrier.sh and deploy.sh

TODO: needs more documentation...
See deploy scripts under [installation/trccloud/terraform/azure/cloudboot/deploy](installation/trccloud/terraform/azure/cloudboot/deploy/)

deploycarrier.sh (for trc-vault-carrier-plugin)
deploy.sh (for trcsh)

### Installing TrcDb atrium for Tierceron Vault
Run trcconfig under [installation/trccloud/terraform/azure/cloudboot](installation/trccloud/terraform/azure/cloudboot)

deploy.sh (for trc-vault-plugin)

TODO: needs more documentation...
See deploy scripts under (installation/trccloud/terraform/azure/cloudboot/deploy/)

## Command line building (via Makefile)
Install build support (Makefile, gcc, etc...):  
```
sudo apt-get install build-essential mingw-w64 
```

Install g3n support libraries (Required for optional tools spiralis and fenestra under atrium):  
```
sudo apt-get install xorg-dev libgl1-mesa-dev libopenal1 libopenal-dev libvorbis0a libvorbis-dev libvorbisfile3  
```


### Automated tests ⚙
TODO...


