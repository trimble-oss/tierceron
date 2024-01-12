# Introduction 
The folders herein contain templates utilized for installing an active vault instance
in the cloud or on a machine (anywhere) capable of storing all your secrets.  You could skip
the trccloud step if you want to install vault yourself on a machine somewhere.  This would
be trivial to do.

# Prerequisites
You must have 4 basic tools installed either via the Makefile or via:
go install github.com/trimble-oss/tierceron/cmd/trcpub@latest
go install github.com/trimble-oss/tierceron/cmd/trcx@latest
go install github.com/trimble-oss/tierceron/cmd/trcinit@latest
go install github.com/trimble-oss/tierceron/cmd/trcconfig@latest

# Installation order
1. trccloud -- cloud infrastructure including an installation of vault. (Optional if you chose manual installation of vault or installation of local vault)
2. trcvault -- initialization of vault and tokens for interacting with vault.
3. trcdb (optional) -- creates the flow database that interacts directly with vault for all it's secrets and data.
   -- This mariadb compliant database runs as a plugin side by side with your vault instance.
   -- No secrets are store on the filesystem anywhere at any time.
4. trcagent (optional)
   -- infrastructure for deployments