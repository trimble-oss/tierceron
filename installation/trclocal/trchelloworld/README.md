# Introduction 
This is a very simple Hello World project illustrating templatization of parameters using the two different types.  It includes a self deployment script executed via the trcsh agent.  Select one of the next two examples that fits your deployment infrastructure.

# To kubernetes deploy a service, copy the kubeexample deploy template example
This example utilizes a secure trch to deploy the service to a kubernetes cluster.
```
mv trc_templates/HelloProject/HelloService/deploy/kubexample/deploy.trc.tmpl trc_templates/HelloProject/HelloService/deploy/
rmdir trc_templates/HelloProject/HelloService/deploy/kubexample/
rm -r trc_templates/HelloProject/HelloService/deploy/serviceexample
```

# To remotely deploy this service, copy the serviceexample deploy template example
This example remotely executes the deploy.trc installation script via a remote trcsh daemon.

```
mv trc_templates/HelloProject/HelloService/deploy/serviceexample/deploy.trc.tmpl trc_templates/HelloProject/HelloService/deploy/
rmdir trc_templates/HelloProject/HelloService/deploy/serviceexample/
rm -r trc_templates/HelloProject/HelloService/deploy/kubeexample
```

# Create a seed work space
```
mkdir trc_seeds
trcx -env=dev -novault
```

# Change some secrets 
```
vim trc_seeds/dev/dev_seed.yml
```

```
trcpub -env=dev -token=$VAULT_TOKEN -addr=$VAULT_ADDR
trcinit -env=dev -token=$VAULT_TOKEN -addr=$VAULT_ADDR
```

# Install to a local service using the trcsh drone
You can define a serivce for the trcsh drone to deploy by executing the script: ./bin/servicedefine.sh
Alternatively you can directly define the service below.

```
trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=dev -defineService -pluginName=trchelloworld -projectservice="HelloProject/HelloService" -pluginType=trcshservice -serviceName="trchelloworld" -codeBundle=trchelloworld -deployroot=/usr/local/hello -deploysubpath=bin
```


# Clean up after yourself
```
rm -r trc_seeds/dev
```
