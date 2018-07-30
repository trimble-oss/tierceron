package server

import (
	"bytes"
	"context"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"bitbucket.org/dexterchaney/whoville/utils"
	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
	sys "bitbucket.org/dexterchaney/whoville/vault-helper/system"
	il "bitbucket.org/dexterchaney/whoville/vault-init/initlib"
	pb "bitbucket.org/dexterchaney/whoville/webapi/rpc/apinator"
)

const tokenPath string = "token_files"
const policyPath string = "policy_files"
const templatePath string = "template_files"

//InitVault Takes init request and inits/seeds vault with contained file data
func (s *Server) InitVault(ctx context.Context, req *pb.InitReq) (*pb.InitResp, error) {
	logBuffer := new(bytes.Buffer)
	logger := log.New(logBuffer, "[INIT]", log.LstdFlags)

	fmt.Println("Initing vault")

	v, err := sys.NewVault(s.VaultAddr, s.CertPath)
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		utils.LogErrorObject(err, logger, false)
		return &pb.InitResp{
			Success: false,
			Logfile: b64.StdEncoding.EncodeToString(logBuffer.Bytes()),
			Tokens:  nil,
		}, err
	}

	// Init and unseal vault
	keyToken, err := v.InitVault(1, 1)
	if err != nil {
		utils.LogErrorObject(err, logger, false)
		return &pb.InitResp{
			Success: false,
			Logfile: b64.StdEncoding.EncodeToString(logBuffer.Bytes()),
			Tokens:  nil,
		}, err
	}
	v.SetToken(keyToken.Token)
	v.SetShards(keyToken.Keys)
	s.VaultToken = keyToken.Token
	//check error returned by unseal
	v.Unseal()
	if err != nil {
		utils.LogErrorObject(err, logger, false)
		return &pb.InitResp{
			Success: false,
			Logfile: b64.StdEncoding.EncodeToString(logBuffer.Bytes()),
			Tokens:  nil,
		}, err
	}

	logger.Printf("Succesfully connected to vault at %s\n", s.VaultAddr)

	// Create engines
	il.CreateEngines(v, logger)

	for _, seed := range req.Files {
		fBytes, err := b64.StdEncoding.DecodeString(seed.Data)
		if err != nil {
			utils.LogErrorObject(err, logger, false)
			return &pb.InitResp{
				Success: false,
				Logfile: b64.StdEncoding.EncodeToString(logBuffer.Bytes()),
				Tokens:  nil,
			}, err
		}
		il.SeedVaultFromData(fBytes, s.VaultAddr, s.VaultToken, seed.Env, s.CertPath, logger)
	}

	il.UploadPolicies(policyPath, v, logger)

	tokens := il.UploadTokens(tokenPath, v, logger)
	tokenMap := map[string]interface{}{}
	for _, token := range tokens {
		tokenMap[token.Name] = token.Value
	}

	mod, err := kv.NewModifier(s.VaultToken, s.VaultAddr, s.CertPath)
	utils.LogErrorObject(err, logger, false)

	mod.Env = "bamboo"
	warn, err := mod.Write("super-secrets/tokens", tokenMap)
	utils.LogErrorObject(err, logger, false)
	utils.LogWarningsObject(warn, logger, false)

	mod.Env = "dev"
	err, warn = il.UploadTemplateDirectory(mod, templatePath)
	utils.LogErrorObject(err, logger, false)
	utils.LogWarningsObject(warn, logger, false)

	mod.Env = "QA"
	err, warn = il.UploadTemplateDirectory(mod, templatePath)
	utils.LogErrorObject(err, logger, false)
	utils.LogWarningsObject(warn, logger, false)

	err = v.EnableAppRole()
	utils.LogErrorObject(err, logger, false)

	err = v.CreateNewRole("bamboo", &sys.NewRoleOptions{
		TokenTTL:    "10m",
		TokenMaxTTL: "15m",
		Policies:    []string{"bamboo"},
	})
	utils.LogErrorObject(err, logger, false)

	roleID, err := v.GetRoleID("bamboo")
	utils.LogErrorObject(err, logger, false)

	secretID, err := v.GetSecretID("bamboo")
	utils.LogErrorObject(err, logger, false)

	logger.SetPrefix("[AUTH]")
	logger.Printf("Role ID: %s\n", roleID)
	logger.Printf("Secret ID: %s\n", secretID)

	s.Log.Println("Init Log \n" + logBuffer.String())
	s.InitGQL()

	res, err := s.APILogin(ctx, &pb.LoginReq{Username: req.Username, Password: req.Password, Environment: req.Env})
	if err != nil {
		utils.LogErrorObject(err, logger, false)
		tokens = append(tokens, &pb.InitResp_Token{
			Name:  "Auth",
			Value: "invalid",
		})
	} else if !res.Success {
		tokens = append(tokens, &pb.InitResp_Token{
			Name:  "Auth",
			Value: "invalid",
		})
	} else {
		tokens = append(tokens, &pb.InitResp_Token{
			Name:  "Auth",
			Value: res.AuthToken,
		})
	}

	return &pb.InitResp{
		Success: true,
		Logfile: b64.StdEncoding.EncodeToString(logBuffer.Bytes()),
		Tokens:  tokens,
	}, nil
}

//APILogin Verifies the user's login with the cubbyhole
func (s *Server) APILogin(ctx context.Context, req *pb.LoginReq) (*pb.LoginResp, error) {
	mod, err := kv.NewModifier(s.VaultToken, s.VaultAddr, s.CertPath)
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}
	mod.Env = req.Environment

	connectionURL, err := mod.ReadValue("apiLogins/meta", "authEndpoint")
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}
	fmt.Printf("Connection url: %s\n", connectionURL)

	credentials := bytes.NewBuffer([]byte{})
	err = json.NewEncoder(credentials).Encode(map[string]string{
		"username": req.Username,
		"password": req.Password,
	})

	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}

	client := &http.Client{}
	res, err := client.Post(connectionURL, "application/json", credentials)

	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}

	if res.StatusCode == 401 {
		return &pb.LoginResp{
			Success:   false,
			AuthToken: "",
		}, nil
	} else if res.StatusCode == 200 || res.StatusCode == 204 {
		var response map[string]interface{}
		bodyBytes, err := ioutil.ReadAll(res.Body)
		if err != nil {
			utils.LogErrorObject(err, s.Log, false)
			return nil, err
		}

		err = json.Unmarshal([]byte(bodyBytes), &response)
		if err != nil {
			utils.LogErrorObject(err, s.Log, false)
			return nil, err
		}

		if name, ok := response["firstName"].(string); ok {
			if id, ok := response["operatorCode"].(string); ok {
				token, err := s.generateJWT(name, id, mod)
				if err != nil {
					utils.LogErrorObject(err, s.Log, false)
					return nil, err
				}

				return &pb.LoginResp{
					Success:   true,
					AuthToken: token,
				}, nil
			}
			err = fmt.Errorf("Unable to parse operatorCode in auth response")
			utils.LogErrorObject(err, s.Log, false)
		} else {
			err = fmt.Errorf("Unable to parse firstName in auth response")
			utils.LogErrorObject(err, s.Log, false)
		}

		return &pb.LoginResp{
			Success:   false,
			AuthToken: "",
		}, err
	}
	err = fmt.Errorf("Unexpected response code from auth endpoint: %d", res.StatusCode)
	utils.LogErrorObject(err, s.Log, false)
	return &pb.LoginResp{
		Success:   false,
		AuthToken: "",
	}, nil
}

//GetStatus requests version info and whether the vault has been initailized
func (s *Server) GetStatus(ctx context.Context, req *pb.NoParams) (*pb.VaultStatus, error) {
	v, err := sys.NewVault(s.VaultAddr, s.CertPath)
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}

	status, err := v.GetStatus()
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}
	return &pb.VaultStatus{
		Version:     status["version"].(string),
		Initialized: status["initialized"].(bool),
		Sealed:      status["sealed"].(bool),
	}, nil
}

//Unseal passes the unseal key to the vault and tries to unseal the vault
func (s *Server) Unseal(ctx context.Context, req *pb.UnsealReq) (*pb.UnsealResp, error) {
	v, err := sys.NewVault(s.VaultAddr, s.CertPath)
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}

	v.AddShard(req.UnsealKey)
	prog, need, sealed, err := v.Unseal()

	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}
	if sealed {
		s.Log.Printf("%d/%d unseal shards\n", prog, need)
	} else {
		s.Log.Println("Vault successfully unsealed")
	}
	return &pb.UnsealResp{
		Sealed:   sealed,
		Progress: int32(prog),
		Needed:   int32(need),
	}, nil
}
