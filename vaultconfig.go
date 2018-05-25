package main

import (
	"flag"
	"fmt"
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
	target := "C:/Users/Sara.wille/workspace/go/src/whoville/experiment"
	file, err := os.Create(target)
	if err != nil {
		log.Println("create file: ", err)
		return
	}
	str := utils.PopulateTemplate(map1, map2, file, template)
	fmt.Println(str)
}
