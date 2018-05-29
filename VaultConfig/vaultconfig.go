package main

import (
	"flag"
	"log"
	"whoville/VaultConfig/utils"
	"whoville/vault-helper/kv"
)

func main() {
	flag.String("accesstoken", "", "vault access token")
	flag.Parse()
	map1 := map[string]interface{}{"key": "value", "fake": "real"}
	map2 := map[string]interface{}{"password": "456", "username": "user"}
	//template := "myKey1: {{.PublicData.key}} fake: {{.PublicData.fake}} username: {{.PrivateData.username}} password: {{.PrivateData.password}}"
	starter := "C:/Users/Sara.wille/workspace/go/src/whoville/VaultConfig/experiment.yaml"
	target := "C:/Users/Sara.wille/workspace/go/src/whoville/VaultConfig/experimentOutput.yaml"
	//make modifier
	token := "f260a1f0-8998-caea-0165-7a59e13ab17a"
	accessiblePaths := []string{"secret/private", "secret/public"}
	mod, err := kv.NewModifier(token, "", accessiblePaths)
	if err != nil {
		panic(err)
	}
	//unable to write for some reason...
	_, err = mod.Write(map2, "secret/private")
	if err != nil {
		panic(err)
	}
	_, err = mod.Write(map1, "secret/public")
	if err != nil {
		panic(err)
	}
	if err != nil {
		log.Println("create modifier: ", err)
		return
	}
	utils.ConfigTemplate(mod, starter, target, "secret/public", "secret/private")
	//if err != nil {
	//	log.Println("config template: ", err)
	//	return
	//}
	//fmt.Println(str)
}
