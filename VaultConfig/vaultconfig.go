package main

import (
	"errors"
	"flag"

	"bitbucket.org/dexterchaney/whoville/VaultConfig/utils"
	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
)

var environments = [...]string{
	"secret",
	"local",
	"dev",
	"QA"}

func main() {
	tokenPtr := flag.String("token", "", "Vault access token")
	addrPtr := flag.String("addr", "http://127.0.0.1:8200", "API endpoint for the vault")
	startDirPtr := flag.String("templateDir", "vault_templates/", "Template directory")
	endDirPtr := flag.String("endDir", "VaultConfig/", "Directory to put configured templates into")
	certPathPtr := flag.String("certPath", "certs/cert_files/serv_cert.pem", "Path to the server certificate")
	env := flag.String("env", environments[0], "Environment to configure")
	flag.Parse()
	//make modifier
	mod, err := kv.NewModifier(*tokenPtr, *addrPtr, *certPathPtr)
	mod.Env = *env
	if err != nil {
		panic(err)
	}
	//get template paths
	paths := []string{}
	secrets, err := mod.List("templates")
	if err != nil {
		panic(err)
	} else if secrets != nil {
		paths = getPaths(mod, "templates", paths)
	} else {
		panic(errors.New("no paths found from templates engine"))
	}
	//configure templates
	utils.ConfigTemplates(*startDirPtr, *endDirPtr, mod, paths...)
}
func getPaths(mod *kv.Modifier, pathName string, pathList []string) []string {
	secrets, err := mod.List(pathName)
	if err != nil {
		panic(err)
	} else if secrets != nil {
		//not end of path, recurse
		slicey := secrets.Data["keys"].([]interface{})
		for _, pathEnd := range slicey {
			path := pathName + "/" + pathEnd.(string)
			pathList = getPaths(mod, path, pathList)
			//don't add on to paths until you're sure it's an END path
		}
		return pathList
	} else {
		//end of path, append pathname and return
		pathList = append(pathList, pathName)
		return pathList
	}
}
