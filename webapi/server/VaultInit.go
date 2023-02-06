package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log"

	il "github.com/trimble-oss/tierceron/trcinit/initlib"
	eUtils "github.com/trimble-oss/tierceron/utils"
	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"
	sys "github.com/trimble-oss/tierceron/vaulthelper/system"
	pb "github.com/trimble-oss/tierceron/webapi/rpc/apinator"
)

const tokenPath string = "token_files"
const policyPath string = "policy_files"
const templatePath string = "template_files"

// InitVault Takes init request and inits/seeds vault with contained file data
func (s *Server) InitVault(ctx context.Context, req *pb.InitReq) (*pb.InitResp, error) {
	logBuffer := new(bytes.Buffer)
	logger := log.New(logBuffer, "[INIT]", log.LstdFlags)

	fmt.Println("Initing vault")
	config := &eUtils.DriverConfig{ExitOnFailure: false, Log: s.Log}

	v, err := sys.NewVault(false, s.VaultAddr, "nonprod", true, false, false, logger)
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		eUtils.LogErrorObject(config, err, false)
		return &pb.InitResp{
			Success: false,
			Logfile: base64.StdEncoding.EncodeToString(logBuffer.Bytes()),
			Tokens:  nil,
		}, err
	}

	// Init and unseal vault
	keyToken, err := v.InitVault(3, 5)
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		return &pb.InitResp{
			Success: false,
			Logfile: base64.StdEncoding.EncodeToString(logBuffer.Bytes()),
			Tokens:  nil,
		}, err
	}
	v.SetToken(keyToken.Token)
	v.SetShards(keyToken.Keys)
	s.VaultToken = keyToken.Token
	//check error returned by unseal
	v.Unseal()
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		return &pb.InitResp{
			Success: false,
			Logfile: base64.StdEncoding.EncodeToString(logBuffer.Bytes()),
			Tokens:  nil,
		}, err
	}

	logger.Printf("Succesfully connected to vault at %s\n", s.VaultAddr)

	// Create engines
	il.CreateEngines(config, v)

	for _, seed := range req.Files {
		fBytes, err := base64.StdEncoding.DecodeString(seed.Data)
		if err != nil {
			eUtils.LogErrorObject(config, err, false)
			return &pb.InitResp{
				Success: false,
				Logfile: base64.StdEncoding.EncodeToString(logBuffer.Bytes()),
				Tokens:  nil,
			}, err
		}
		il.SeedVaultFromData(&eUtils.DriverConfig{Insecure: false, VaultAddress: s.VaultAddr, Token: s.VaultToken, Env: seed.Env, Log: logger, ExitOnFailure: true, ServicesWanted: []string{""}, WantCerts: true}, "", fBytes)
	}

	il.UploadPolicies(config, policyPath, v, false)

	tokens := il.UploadTokens(config, tokenPath, nil, v)
	tokenMap := map[string]interface{}{}
	for _, token := range tokens {
		tokenMap[token.Name] = token.Value
	}

	mod, err := helperkv.NewModifier(false, s.VaultToken, s.VaultAddr, "nonprod", nil, true, s.Log)
	eUtils.LogErrorObject(config, err, false)

	mod.RawEnv = "bamboo"
	mod.Env = "bamboo"
	warn, err := mod.Write("super-secrets/tokens", tokenMap, config.Log)
	eUtils.LogErrorObject(config, err, false)
	eUtils.LogWarningsObject(config, warn, false)

	envStrings := SelectedEnvironment
	for _, e := range envStrings {
		mod.Env = e
		err, warn = il.UploadTemplateDirectory(mod, templatePath, logger)
		eUtils.LogErrorObject(config, err, false)
		eUtils.LogWarningsObject(config, warn, false)
	}

	err = v.EnableAppRole()
	eUtils.LogErrorObject(config, err, false)

	err = v.CreateNewRole("bamboo", &sys.NewRoleOptions{
		TokenTTL:    "10m",
		TokenMaxTTL: "15m",
		Policies:    []string{"bamboo"},
	})
	eUtils.LogErrorObject(config, err, false)

	roleID, _, err := v.GetRoleID("bamboo")
	eUtils.LogErrorObject(config, err, false)

	secretID, err := v.GetSecretID("bamboo")
	eUtils.LogErrorObject(config, err, false)

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
	s.InitConfig(config, targetEnv)

	res, err := s.APILogin(ctx, &pb.LoginReq{Username: req.Username, Password: req.Password, Environment: targetEnv})
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
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
		Logfile: base64.StdEncoding.EncodeToString(logBuffer.Bytes()),
		Tokens:  tokens,
	}, nil
}

// APILogin Verifies the user's login with the cubbyhole
func (s *Server) APILogin(ctx context.Context, req *pb.LoginReq) (*pb.LoginResp, error) {
	result := pb.LoginResp{
		Success:   false,
		AuthToken: "",
	}
	config := &eUtils.DriverConfig{ExitOnFailure: false, Log: s.Log}

	mod, err := helperkv.NewModifier(false, s.VaultToken, s.VaultAddr, "nonprod", nil, true, s.Log)
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		return &result, err
	}
	mod.Env = req.Environment

	authSuccess, name, err := s.authUser(config, mod, req.Username, req.Password)
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		return &result, err
	}

	token, errJwtGen := s.generateJWT(name, req.Environment+"/"+req.Username, mod)
	if errJwtGen != nil {
		eUtils.LogErrorObject(config, errJwtGen, false)
		return &result, err
	}
	result.AuthToken = token
	result.Success = authSuccess

	return &result, nil
}

// GetStatus requests version info and whether the vault has been initailized
func (s *Server) GetStatus(ctx context.Context, req *pb.NoParams) (*pb.VaultStatus, error) {
	v, err := sys.NewVault(false, s.VaultAddr, "nonprod", true, false, false, s.Log)
	if v != nil {
		defer v.Close()
	}
	config := &eUtils.DriverConfig{ExitOnFailure: false, Log: s.Log}
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		return nil, err
	}

	status, err := v.GetStatus()
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		return nil, err
	}
	return &pb.VaultStatus{
		Version:     status["version"].(string),
		Initialized: status["initialized"].(bool),
		Sealed:      status["sealed"].(bool),
	}, nil
}

// Unseal passes the unseal key to the vault and tries to unseal the vault
func (s *Server) Unseal(ctx context.Context, req *pb.UnsealReq) (*pb.UnsealResp, error) {
	v, err := sys.NewVault(false, s.VaultAddr, "nonprod", false, false, false, s.Log)
	config := &eUtils.DriverConfig{ExitOnFailure: false, Log: s.Log}
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		return nil, err
	}

	v.AddShard(req.UnsealKey)
	prog, need, sealed, err := v.Unseal()

	if err != nil {
		eUtils.LogErrorObject(config, err, false)
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
