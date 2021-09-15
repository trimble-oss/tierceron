package server

import (
	"bytes"
	"context"
	b64 "encoding/base64"
	"fmt"
	"log"

	"Vault.Whoville/utils"
	"Vault.Whoville/vaulthelper/kv"
	sys "Vault.Whoville/vaulthelper/system"
	il "Vault.Whoville/vaultinit/initlib"
	pb "Vault.Whoville/webapi/rpc/apinator"
)

const tokenPath string = "token_files"
const policyPath string = "policy_files"
const templatePath string = "template_files"

//InitVault Takes init request and inits/seeds vault with contained file data
func (s *Server) InitVault(ctx context.Context, req *pb.InitReq) (*pb.InitResp, error) {
	logBuffer := new(bytes.Buffer)
	logger := log.New(logBuffer, "[INIT]", log.LstdFlags)

	fmt.Println("Initing vault")

	v, err := sys.NewVault(false, s.VaultAddr, "nonprod", true, false)
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
		il.SeedVaultFromData(false, fBytes, s.VaultAddr, s.VaultToken, seed.Env, logger, "", true)
	}

	il.UploadPolicies(policyPath, v, false, logger)

	tokens := il.UploadTokens(tokenPath, v, logger)
	tokenMap := map[string]interface{}{}
	for _, token := range tokens {
		tokenMap[token.Name] = token.Value
	}

	mod, err := kv.NewModifier(false, s.VaultToken, s.VaultAddr, "nonprod", nil)
	utils.LogErrorObject(err, logger, false)

	mod.Env = "bamboo"
	warn, err := mod.Write("super-secrets/tokens", tokenMap)
	utils.LogErrorObject(err, logger, false)
	utils.LogWarningsObject(warn, logger, false)

	envStrings := SelectedEnvironment
	for _, e := range envStrings {
		mod.Env = e
		err, warn = il.UploadTemplateDirectory(mod, templatePath, logger)
		utils.LogErrorObject(err, logger, false)
		utils.LogWarningsObject(warn, logger, false)
	}

	err = v.EnableAppRole()
	utils.LogErrorObject(err, logger, false)

	err = v.CreateNewRole("bamboo", &sys.NewRoleOptions{
		TokenTTL:    "10m",
		TokenMaxTTL: "15m",
		Policies:    []string{"bamboo"},
	})
	utils.LogErrorObject(err, logger, false)

	roleID, _, err := v.GetRoleID("bamboo")
	utils.LogErrorObject(err, logger, false)

	secretID, err := v.GetSecretID("bamboo")
	utils.LogErrorObject(err, logger, false)

	s.Log.Println("Init Log \n" + logBuffer.String())

	logger.SetPrefix("[AUTH]")
	logger.Printf("Role ID: %s\n", roleID)
	logger.Printf("Secret ID: %s\n", secretID)

	s.InitGQL()
	var targetEnv string
	for _, e := range envStrings {
		targetEnv = e
		if e == "dev" {
			break
		} else if e == "staging" {
			SelectedEnvironment = SelectedWebEnvironment
			break
		}
	}
	s.InitConfig(targetEnv)

	res, err := s.APILogin(ctx, &pb.LoginReq{Username: req.Username, Password: req.Password, Environment: targetEnv})
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

	if sToken, ok := tokenMap["webapp"].(string); ok {
		s.VaultToken = sToken
	} else {
		s.VaultToken = ""
	}

	return &pb.InitResp{
		Success: true,
		Logfile: b64.StdEncoding.EncodeToString(logBuffer.Bytes()),
		Tokens:  tokens,
	}, nil
}

//APILogin Verifies the user's login with the cubbyhole
func (s *Server) APILogin(ctx context.Context, req *pb.LoginReq) (*pb.LoginResp, error) {
	result := pb.LoginResp{
		Success:   false,
		AuthToken: "",
	}

	mod, err := kv.NewModifier(false, s.VaultToken, s.VaultAddr, "nonprod", nil)
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return &result, err
	}
	mod.Env = req.Environment

	authSuccess, name, err := s.authUser(mod, req.Username, req.Password)
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return &result, err
	}

	token, errJwtGen := s.generateJWT(name, req.Environment+"/"+req.Username, mod)
	if errJwtGen != nil {
		utils.LogErrorObject(errJwtGen, s.Log, false)
		return &result, err
	}
	result.AuthToken = token
	result.Success = authSuccess

	return &result, nil
}

//GetStatus requests version info and whether the vault has been initailized
func (s *Server) GetStatus(ctx context.Context, req *pb.NoParams) (*pb.VaultStatus, error) {
	v, err := sys.NewVault(false, s.VaultAddr, "nonprod", true, false)
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
	v, err := sys.NewVault(false, s.VaultAddr, "nonprod", false, false)
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
