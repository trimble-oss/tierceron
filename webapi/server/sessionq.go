package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"tierceron/utils"
	eUtils "tierceron/utils"
	pb "tierceron/webapi/rpc/apinator"

	configcore "VaultConfig.Bootstrap/configcore"
)

// ProxyLogin proxy logs in the user.
func ProxyLogin(config *eUtils.DriverConfig, authHost string, req *pb.LoginReq) (string, string, *pb.LoginResp, error) {
	credentials := bytes.NewBuffer([]byte{})

	err := json.NewEncoder(credentials).Encode(map[string]string{
		"username": req.Username,
		"password": req.Password,
	})

	if err != nil {
		utils.LogErrorObject(config, err, false)
		return "", "", nil, err
	}

	client := &http.Client{}
	res, err := client.Post(authHost, "application/json", credentials)

	if err != nil {
		utils.LogErrorObject(config, err, false)
		return "", "", nil, err
	}

	if res.StatusCode == 401 {
		return "", "", &pb.LoginResp{
			Success:   false,
			AuthToken: "",
		}, nil
	} else if res.StatusCode == 200 || res.StatusCode == 204 {
		var response map[string]interface{}
		bodyBytes, err := ioutil.ReadAll(res.Body)
		if err != nil {
			utils.LogErrorObject(config, err, false)
			return "", "", nil, err
		}

		err = json.Unmarshal([]byte(bodyBytes), &response)
		if err != nil {
			utils.LogErrorObject(config, err, false)
			return "", "", nil, err
		}

		if userNameField, ok := response[configcore.UserNameField].(string); ok {
			if userCodeField, ok := response[configcore.UserCodeField].(string); ok {

				return userNameField, userCodeField, &pb.LoginResp{
					Success:   true,
					AuthToken: "",
				}, nil
			}
			err = fmt.Errorf("Unable to parse userCodeField in auth response")
			utils.LogErrorObject(config, err, false)
		} else {
			err = fmt.Errorf("Unable to parse userNameField in auth response")
			utils.LogErrorObject(config, err, false)
		}

		return "", "", &pb.LoginResp{
			Success:   false,
			AuthToken: "",
		}, err
	}
	err = fmt.Errorf("Unexpected response code from auth endpoint: %d", res.StatusCode)
	utils.LogErrorObject(config, err, false)
	return "", "", &pb.LoginResp{
		Success:   false,
		AuthToken: "",
	}, nil
}
