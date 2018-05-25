package main

import (
	"flag"
	"log"
	"os"
	"whoville/utils"
)

func main() {
	flag.String("accesstoken", "", "vault access token")
	flag.Parse()
	map1 := map[string]interface{}{"key": "value", "fake": "data"}
	map2 := map[string]interface{}{"password": "123", "username": "user"}
	template := "myKey1: {{.PublicData.key}} fake: {{.PublicData.fake}} username: {{.PrivateData.username}} password: {{.PrivateData.password}}"
	target := "C:/Users/Sara.wille/workspace/src/whoville/experiment"
	file, err := os.Create(target)
	if err != nil {
		log.Println("create file: ", err)
		return
	}
	utils.PopulateTemplate(map1, map2, file, template)
}

//libraries for in memory file
//s3 upload file to bucket
//do we want to directly import template files? (assuming yes)
//s3 buckets
//configinator as a web service -- twitch
//webservice web call: environment and product name -- protobuffs
//multiple auth types? user/pass AND tokens?
//}
