package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	pb "github.com/trimble-oss/tierceron/trcweb/rpc/apinator"
)

// ProxyLogin proxy logs in the user.
func ProxyLogin(config *eUtils.DriverConfig, authHost string, req *pb.LoginReq) (string, string, *pb.LoginResp, error) {
	credentials := bytes.NewBuffer([]byte{})

	err := json.NewEncoder(credentials).Encode(map[string]string{
		"username": req.Username,
		"password": req.Password,
	})

	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		return "", "", nil, err
	}

	client := &http.Client{}
	res, err := client.Post(authHost, "application/json", credentials)

	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		return "", "", nil, err
	}

	switch res.StatusCode {
	case 401:
		return "", "", &pb.LoginResp{
			Success:   false,
			AuthToken: "",
		}, nil
	case 200:
		fallthrough
	case 204:
		var response map[string]interface{}
		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			eUtils.LogErrorObject(config, err, false)
			return "", "", nil, err
		}

		err = json.Unmarshal([]byte(bodyBytes), &response)
		if err != nil {
			eUtils.LogErrorObject(config, err, false)
			return "", "", nil, err
		}

		if userNameField, ok := response[coreopts.BuildOptions.GetUserNameField()].(string); ok {
			if userCodeField, ok := response[coreopts.BuildOptions.GetUserCodeField()].(string); ok {

				return userNameField, userCodeField, &pb.LoginResp{
					Success:   true,
					AuthToken: "",
				}, nil
			}
			err = fmt.Errorf("unable to parse userCodeField in auth response")
			eUtils.LogErrorObject(config, err, false)
		} else {
			err = fmt.Errorf("unable to parse userNameField in auth response")
			eUtils.LogErrorObject(config, err, false)
		}

		return "", "", &pb.LoginResp{
			Success:   false,
			AuthToken: "",
		}, err
	}
	err = fmt.Errorf("unexpected response code from auth endpoint: %d", res.StatusCode)
	eUtils.LogErrorObject(config, err, false)
	return "", "", &pb.LoginResp{
		Success:   false,
		AuthToken: "",
	}, nil
}
