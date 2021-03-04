package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"Vault.Whoville/utils"
	pb "Vault.Whoville/webapi/rpc/apinator"
)

const activeSessionsQuery = `select distinct User_Name, Time_Logged_On from PA_COMPANY_CODE cc1 with (nolock)
								join (select distinct Operator_ID from PA_OPERATOR_MASTER with(nolock)) om
								on  cc1.User_Id = om.Operator_Id 
								and Time_Logged_On = (select max(Time_Logged_On) from PA_COMPANY_CODE cc2 with (nolock) where cc1.User_Name=cc2.User_Name)`

const authQuery = `select top(1)
                      Operator_ID, Operator_Name, Password_Hash, Salt, Iteration_Count
                      from PA_OPERATOR_MASTER with(nolock)
                      where Operator_ID = @Id;`

// GetActiveSessionQuery - returns a SQL query with a user name and time logged in.
func GetActiveSessionQuery() string {
	return activeSessionsQuery
}

// GetAuthLoginQuery - returns a SQL query to retrieve operator.
func GetAuthLoginQuery() string {
	return authQuery
}

// ProxyLogin proxy logs in the user.
func ProxyLogin(authHost string, req *pb.LoginReq, log *log.Logger) (string, string, *pb.LoginResp, error) {
	credentials := bytes.NewBuffer([]byte{})

	err := json.NewEncoder(credentials).Encode(map[string]string{
		"username": req.Username,
		"password": req.Password,
	})

	if err != nil {
		utils.LogErrorObject(err, log, false)
		return "", "", nil, err
	}

	client := &http.Client{}
	res, err := client.Post(authHost, "application/json", credentials)

	if err != nil {
		utils.LogErrorObject(err, log, false)
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
			utils.LogErrorObject(err, log, false)
			return "", "", nil, err
		}

		err = json.Unmarshal([]byte(bodyBytes), &response)
		if err != nil {
			utils.LogErrorObject(err, log, false)
			return "", "", nil, err
		}

		if userName, ok := response["firstName"].(string); ok {
			if operatorID, ok := response["operatorCode"].(string); ok {

				return userName, operatorID, &pb.LoginResp{
					Success:   true,
					AuthToken: "",
				}, nil
			}
			err = fmt.Errorf("Unable to parse operatorCode in auth response")
			utils.LogErrorObject(err, log, false)
		} else {
			err = fmt.Errorf("Unable to parse firstName in auth response")
			utils.LogErrorObject(err, log, false)
		}

		return "", "", &pb.LoginResp{
			Success:   false,
			AuthToken: "",
		}, err
	}
	err = fmt.Errorf("Unexpected response code from auth endpoint: %d", res.StatusCode)
	utils.LogErrorObject(err, log, false)
	return "", "", &pb.LoginResp{
		Success:   false,
		AuthToken: "",
	}, nil
}
