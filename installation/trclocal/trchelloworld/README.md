# Introduction 
This is a much simpler Hello World project illustrating templatization of parameters using the two different types.

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
trcinit -env=dev -token=$VAULT_TOKEN -addr=$VAULT_ADDR
```

# Clean up after yourself
```
rm -r trc_seeds/dev
```
