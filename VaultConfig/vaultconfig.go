package main

import (
	"flag"
	"log"

	"bitbucket.org/dexterchaney/whoville/VaultConfig/utils"
	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
)

func main() {
	tokenPtr := flag.String("token", "", "Vault access token")
	addrPtr := flag.String("addr", "http://127.0.0.1:8200", "API endpoint for the vault")
	startDirPtr := flag.String("templateDir", "", "Template directory")
	endDirPtr := flag.String("endDir", "", "Configured template directory")

	flag.Parse()
	map1 := map[string]interface{}{"sendGridApiKey": "apikey", "password": "456", "username": "user"}
	map2 := map[string]interface{}{"keyStorePass": "randomPass", "keyStorePath": "randomPath"}
	//make modifier
	//pass in host, token, target directories?
	//use policies that max put in
	mod, err := kv.NewModifier(*tokenPtr, *addrPtr)
	if err != nil {
		panic(err)
	}
	//unable to write for some reason...
	_, err = mod.Write("secret/hibernate", map1)
	if err != nil {
		panic(err)
	}
	_, err = mod.Write("secret/config", map2)
	if err != nil {
		log.Println("create modifier: ", err)
		return
	}
	utils.ConfigTemplates(*startDirPtr, *endDirPtr, mod, "secret/config", "secret/hibernate")
}
