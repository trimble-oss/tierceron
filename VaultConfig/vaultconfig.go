package main

import (
	"flag"
	"log"

	"bitbucket.org/dexterchaney/whoville/VaultConfig/utils"
	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
)

func main() {
	flag.String("accesstoken", "", "vault access token")
	flag.Parse()
	map1 := map[string]interface{}{"sendGridApiKey": "apikey", "password": "456", "username": "user"}
	map2 := map[string]interface{}{"keyStorePass": "randomPass", "keyStorePath": "randomPath"}
	//template := "myKey1: {{.PublicData.key}} fake: {{.PublicData.fake}} username: {{.PrivateData.username}} password: {{.PrivateData.password}}"
	starter1 := "C:/Users/Sara.wille/workspace/go/src/bitbucket.org/dexterchaney/whoville/vault_templates/ST/hibernate.properties.tmpl"
	starter2 := "C:/Users/Sara.wille/workspace/go/src/bitbucket.org/dexterchaney/whoville/vault_templates/ST/config.yaml.tmpl"
	target1 := "C:/Users/Sara.wille/workspace/go/src/bitbucket.org/dexterchaney/whoville/VaultConfig/experimentOutput1.yaml"
	target2 := "C:/Users/Sara.wille/workspace/go/src/bitbucket.org/dexterchaney/whoville/VaultConfig/experimentOutput2.yaml"
	//make modifier
	token := "439d53ec-ad66-37a5-5272-012f9bf1794d"
	mod, err := kv.NewModifier(token, "")
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
	utils.ConfigTemplate(mod, starter1, target1, "secret/hibernate")
	utils.ConfigTemplate(mod, starter2, target2, "secret/config")
	//if err != nil {
	//	log.Println("config template: ", err)
	//	return
	//}
	//fmt.Println(str)
}
