package main

import (
	"flag"
	"fmt"

	"bitbucket.org/dexterchaney/whoville/VaultConfig/utils"
	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
)

func main() {

	tokenPtr := flag.String("token", "", "Vault access token")
	addrPtr := flag.String("addr", "http://127.0.0.1:8200", "API endpoint for the vault")
	startDirPtr := flag.String("templateDir", "C:/Users/Sara.wille/workspace/go/src/bitbucket.org/dexterchaney/whoville/vault_templates/ST/", "Template directory")
	endDirPtr := flag.String("endDir", "C:/Users/Sara.wille/workspace/go/src/bitbucket.org/dexterchaney/whoville/VaultConfig/", "Directory to put configured templates into")
	flag.Parse()
	//make modifier
	//pass in host, token, target directories?
	//use policies that max put in
	mod, err := kv.NewModifier(*tokenPtr, *addrPtr)
	if err != nil {
		panic(err)
	}
	//engines := []string{"super-secrets", "templates", "value-metrics"} //, "value-metrics"} //"templates"
	paths := []string{}
	//find a way to list paths corresponding to templates/super-secrets/value-metrics
	secrets, err := mod.List("templates")
	if err != nil {
		panic(err)
	} else if secrets != nil {
		paths = getPaths(mod, "templates", paths)
	} else {
		fmt.Println("no paths found from templates engine")
	}
	//now we need to check if these paths have any more paths leading from them.
	fmt.Println(paths)
	//paths := []string{"super-secrets/KeyStore", "super-secrets/SendGrid", "super-secrets/SpectrumDB"}
	utils.ConfigTemplates(*startDirPtr, *endDirPtr, mod, paths...)
}
func getPaths(mod *kv.Modifier, pathName string, pathList []string) []string {
	secrets, err := mod.List(pathName)
	if err != nil {
		panic(err)
	} else if secrets != nil {
		slicey := secrets.Data["keys"].([]interface{})
		for _, pathEnd := range slicey {
			path := pathName + "/" + pathEnd.(string)
			pathList = getPaths(mod, path, pathList)
			//don't add on to paths until you're sure it's an END path
		}
		return pathList
	} else {
		fmt.Println("adding path ", pathName)
		pathList = append(pathList, pathName)
		return pathList
	}
}
