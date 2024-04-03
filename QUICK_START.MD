
## Let's get started 🔧 

##Command line building (via Makefile)
Install build support (Makefile, gcc, etc...):
sudo apt-get install build-essential

Install g3n support libraries (Required for optional tools spiralis and fenestra):
sudo apt-get install xorg-dev libgl1-mesa-dev libopenal1 libopenal-dev libvorbis0a libvorbis-dev libvorbisfile3

#Deploy a cloud hosted vault instance using Tierceron
First, we'll need a utility to help us generate some infrastructure in Azure.
Everything in Tierceron run directly from a Hashicorp Vault instance and it
is easiest if you set one up in Azure cloud.  You can use a tiny instance
to get started.
go get github.com/trimble-oss/tierceron/cmd/trcconfig
go get github.com/trimble-oss/tierceron/cmd/trcx

Now that we have the tools we need (trcconfig and trcx), go to the terraform templates.
cd terraform/azure/trcagent
trcconfig -novault

This next step generates a seed file for use in managing your sensitive terraform
configurations in Vault.
mkdir trc_seeds
trcx -novault

Edit trc_seeds/dev/dev_seed.yml using your favorite editor to customize the
configurations you'll want to use in your terraform script.

##Terraform set-up
[Install Terraform](https://www.terraform.io/downloads.html)

From command line, navigate to the directory containing terraform files (terraform/azure/trcagent) and run `terraform init`
cd terraform/azure/trcagent

##What you'll need
deploy.pem file

serv_key.pem file

vpc id

subnets

##Deploying Vault
Now run `terraform plan -out=vault.plan`
Terraform should ask you for the deploy.pem path, vpc id, and subnets.
To deploy the vault instance, run `terraform apply "vault.plan"`

#Get your vault instance running in the cloud.
Go to the Cloud Console and navigate to the Virtual Machine instance named vault.
Copy the private IP (at this point, you could add routing for the private IP)

ssh -i deploy.pem username@ip
service vault start

Connect to https://<vaulthost>:<vaultport>/v1/sys/health to check that the vault is running.

To destroy the instance of vault, run `terraform destroy` or terminate it from the AWS Console.

#Initialize your vault instance so it can be ready to start accepting some secret data.
To initialize vault, you'll first need another tool to create a usable vault instance.
go get github.com/trimble-oss/tierceron/cmd/trcinit



##Connecting to the instance
Navigate to the EC2 instance of vault on AWS and get the Public DNS.
Connect to the instance from the directory containing your deploy.pem file with the Public DNS, for example: `ssh -i "deploy.pem" ubuntu@ec2-xx-xxx-xx-xxx.us-west-2.compute.amazonaws.com`

##Configuring Cloud credentials
Configure the cloud credentials for the instance using either:
  `sudo az configure ` TODO
  `sudo aws configure --profile default`

It will ask for your cloud credentials. Use an appropriate region for the default region and "json" for default output format.

##Seeding Vault 
Drop vault mysql backing store database in DBVisualizer
Stop vault service
Start vault service
Run sql command: `alter table vault modify vault_key varbinary(1024);`
Connect to <vaulthost>
Seed Vault by uploading seed files
Copy tokens out of apiRouter standard output 
Exit/kill apiRouter 
Start trcapi service
Log into UI using webapi token generated from seed process

#Initialize Vault

##Create Policy Files
Policies are written in hcl files. Write policies according to [Hashicorp's guide](https://www.vaultproject.io/docs/concepts/policies.html) and place them in the same folder. 
Policies will be named the by their file name (ex: `mypolicy.hcl` is written as `mypolicy`)

##Create Token Files
Access tokens are written in YAML files. A full list of configurable parameters can be found on [Vault's Website](https://www.vaultproject.io/api/auth/token/index.html#parameters).
Tokens can only be created when logged in to the vault with token creation permissions.

##Create Seed Files
Seed files are written in a YAML format. Each seed file is seeded under a single environment. 
Seperate values for different environments in different files. [Example](tierceron/trc_seeds/seed.yml)
Data is organized in the vault as key-values pairs in a file system. The seed file should reflect this organization. 
All values in the seed fall under 5 'root' categories.

###templates
The trc_templates folder should be seperated by service, then file. The keys match the name of what is being substituted in the 
template file (e.g. `{{.username}}`). The value for each key provides a reference to where the secret is stored in the vault. 
This allows viewing what's stored in the vault without seeing the actual value. The value is a 2 element array. The first 
entry is the location of the secret in the vault, the second entry is the key for the secret.

Example:
```
templates:
	ProjectName:
		hibernate:
			username: [super-secrets/ProjectName, user]
			password: [super-secrets/ProjectName, pass]
```

###super-secrets
This is where sensitive information is stored. Data in `super-secrets` is only given on a need to know basis. This should be restricted to populating templates.
Secrets should be grouped by use cases (e.g. database credentials, API key, certificate info). The values referenced in `templates` will populate the template files
exactly as they appear here.

Example:
```
super-secrets:
	ProjectName:
		url: jdbc:sqlserver://db.database.com;DatabaseName=DB
		user: MyUsername
		pass: ThePassword
	SendGrid:
		ApiKey: <Key>
```

###verification
This is used in the seeding process to check the validity of secrets values. To verify a `super-secret`, create an entry under `verification` with the same name.  
Each entry should have the key `type` to determine the type of credential to verify. `type` can be

*	**db**  
	Used for database credentials. Requires `url`, `user`, `pass`
	
*	**KeyStore**  _TODO_  
	Verifies KeyStore password. Requires `verify-path` and `Pass`. Keystore file should be found in the folder referenced by `verify-path`
	
*  	**SendGridKey**  _TODO_  
	Used for verifying SendGrid API key. Requires `ApiKey`
	
###values and value-metrics
These should not be seeded and are used by the build tools to track configuration data. However, values and metrics can be seeded if needed.  
_Note: Values seeded here may be overwritten by configuration tools_

#Run Trc Init 
Token is the only flag that needs to be specified to run. Also, when running trcinit in the local environment, user will be prompted for environment, username, and password.

### Commands
```
trcinit
	arguments:
	
	-addr string
		API endpoint for the vault (default "http://localhost:8200")
		Specified in vault's config file
	-dev
		Vault server running in dev mode (does not need to be unsealed)
		Only used for testing. Command should not be used in production
	-env string
		Environment to be seeded (default "dev")
		All seed files will be placed under this environment
	-log string
		Output path for log files (default "/var/log/trcinit.log")
	-new
		New vault being initialized. Creates engines and requests first-time initialization
		Only use this when creating the vault for the first time. Omit if reseeding
	-policies string
		Directory that contains vault policies (default "policy_files")
	-seeds string
		Directory that contains tierceron seeds (default "trc_seeds")
	-shard string
		Key shard used to unseal a vault that has been initialized but restarted
		This is required when the vault is restarted but the data has persisted
	-token string
		Vault access token, only use if in dev mode or reseeding
	-tokenfiles string
		Directory with yaml files used to generate tokens (default "token_files")
```
#Run Trc Config
Run in the ServiceTechAPI folder and check the hibernate.properties file generated by trcconfig to verify that the vault has been reseeded.
Will not run if you don't have appRoleID and secretID tokens. 

**CLI Syntax:**

*	`trcconfig` 
    Executes trcconfig. If the cert file exists trcconfig will use appRoleID and secretID tokens from the file. If no approle file exists trcconfig will prompt for appRoleID and secretID then create a cert file and save those tokens to it for future use.

*	`trcconfig -appRoleID=foo -secretID=bar`
     Overrides the tokens in the cert file. The new tokens will be saved for future use.

*	`trcconfig -h`
    Prints command menu.

### Commands
```
trcconfig
	arguments:
	-token string
		Vault access token
	-addr string
		API endpoint for the vault (default "http://localhost:8200")
		Specified in vault's config file
	-startDir string
		Template directory (default "trc_templates")
	-endDir string
		Directory to put configured templates into
	-secretMode bool
		Only override secret values in templates? (default "true")
	-env string
		Environment to configure (default "dev")
		All seed files will be placed under this environment
	-servicesWanted string
		Services to pull template values for, in the form 'service1,service2' (defaults to all services)
	-secretID string
		Secret for app role ID
	-appRoleID string
		Public app role ID
	-tokenfiles string
		Directory with yaml files used to generate tokens (default "token_files")
	-tokenName string
		Token name used by this trcconfig to access the trc
```

### Project structure ❏

Until this project becomes Open source, in order to build Tierceron, you will need to do the following:

$ git config --global url."git@github.com:trimble-oss/".insteadOf "https://github.com/trimble-oss/"
$ eval `ssh-agent`
# go get needs the following to function seemlessly with Private/Protected Trimble repositories.
# id_rsa should be the private key that has previously been given access to Trimble-oss.
$ ssh-add ~/.ssh/id_rsa_git
