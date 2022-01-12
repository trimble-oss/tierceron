package util

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	sys "tierceron/vaulthelper/system"

	"github.com/txn2/txeh"
)

func GetLocalVaultHost() (string, error) {
	vaultHost := "https://"
	vaultErr := errors.New("No usable local vault found.")
	hostFileLines, pherr := txeh.ParseHosts("/etc/hosts")
	if pherr != nil {
		return "", pherr
	}

	for _, hostFileLine := range hostFileLines {
		for _, host := range hostFileLine.Hostnames {
			if (strings.Contains(host, "whoboot.org") || strings.Contains(host, "dexchadev.org") || strings.Contains(host, "dexterchaney.com")) && strings.Contains(hostFileLine.Address, "127.0.0.1") {
				vaultHost = vaultHost + host
				break
			}
		}
	}

	// Now, look for vault.
	for i := 8190; i < 8300; i++ {
		vh := vaultHost + ":" + strconv.Itoa(i)
		_, err := sys.NewVault(true, vh, "", false, true, true)
		if err == nil {
			vaultHost = vaultHost + ":" + strconv.Itoa(i)
			vaultErr = nil
			break
		}
	}

	return vaultHost, vaultErr
}

func GetJSONFromClient(httpClient *http.Client, address string, body io.Reader) map[string]interface{} {
	var jsonData map[string]interface{}
	request, err := http.NewRequest("POST", address, body)
	if err != nil {
		panic(err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	response, err := httpClient.Do(request)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()

	// read json http response
	jsonDataFromHttp, err := ioutil.ReadAll(response.Body)

	if err != nil {
		panic(err)
	}

	err = json.Unmarshal([]byte(jsonDataFromHttp), &jsonData)

	if err != nil {
		panic(err)
	}

	return jsonData
}
