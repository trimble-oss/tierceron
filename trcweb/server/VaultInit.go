package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log"

	"github.com/trimble-oss/tierceron/pkg/core"
	"github.com/trimble-oss/tierceron/pkg/core/cache"
	il "github.com/trimble-oss/tierceron/pkg/trcinit/initlib"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
	sys "github.com/trimble-oss/tierceron/pkg/vaulthelper/system"
	pb "github.com/trimble-oss/tierceron/trcweb/rpc/apinator"
)

const tokenPath string = "token_files"
const policyPath string = "policy_files"
const templatePath string = "template_files"

// InitVault Takes init request and inits/seeds vault with contained file data
func (s *Server) InitVault(ctx context.Context, req *pb.InitReq) (*pb.InitResp, error) {
	logBuffer := new(bytes.Buffer)
	logger := log.New(logBuffer, "[INIT]", log.LstdFlags)

	fmt.Println("Initing vault")
	coreConfig := &core.CoreConfig{ExitOnFailure: false, Log: s.Log}

	v, err := sys.NewVault(false, s.VaultAddrPtr, "nonprod", true, false, false, logger)
	if err != nil {
		eUtils.LogErrorObject(coreConfig, err, false)
		eUtils.LogErrorObject(coreConfig, err, false)
		return &pb.InitResp{
			Success: false,
			Logfile: base64.StdEncoding.EncodeToString(logBuffer.Bytes()),
			Tokens:  nil,
		}, err
	}

	// Init and unseal vault
	keyToken, err := v.InitVault(3, 5)
	if err != nil {
		eUtils.LogErrorObject(coreConfig, err, false)
		return &pb.InitResp{
			Success: false,
			Logfile: base64.StdEncoding.EncodeToString(logBuffer.Bytes()),
			Tokens:  nil,
		}, err
	}
	v.SetToken(keyToken.TokenPtr)
	v.SetShards(keyToken.Keys)
	s.VaultTokenPtr = keyToken.TokenPtr
	//check error returned by unseal
	v.Unseal()
	if err != nil {
		eUtils.LogErrorObject(coreConfig, err, false)
		return &pb.InitResp{
			Success: false,
			Logfile: base64.StdEncoding.EncodeToString(logBuffer.Bytes()),
			Tokens:  nil,
		}, err
	}

	logger.Printf("Successfully connected to vault at %s\n", s.VaultAddrPtr)

	// Create engines
	il.CreateEngines(coreConfig, v)

	for _, seed := range req.Files {
		fBytes, err := base64.StdEncoding.DecodeString(seed.Data)
		if err != nil {
			eUtils.LogErrorObject(coreConfig, err, false)
			return &pb.InitResp{
				Success: false,
				Logfile: base64.StdEncoding.EncodeToString(logBuffer.Bytes()),
				Tokens:  nil,
			}, err
		}

		il.SeedVaultFromData(&config.DriverConfig{
			CoreConfig: &core.CoreConfig{
				WantCerts: true,
				Insecure:  false,
				TokenCache: cache.NewTokenCache(fmt.Sprintf("config_token_%s_unrestricted", seed.Env),
					s.VaultTokenPtr,
					s.VaultAddrPtr),
				Env:           seed.Env,
				ExitOnFailure: true,
				Log:           logger,
			},
			ServicesWanted: []string{""}}, "", fBytes)
	}

	il.UploadPolicies(coreConfig, policyPath, v, false)

	tokens := il.UploadTokens(coreConfig, tokenPath, nil, v)
	tokenMap := map[string]interface{}{}
	for _, token := range tokens {
		tokenMap[token.Name] = token.Value
	}

	mod, err := helperkv.NewModifier(false, s.VaultTokenPtr, s.VaultAddrPtr, "nonprod", nil, true, s.Log)
	eUtils.LogErrorObject(coreConfig, err, false)

	mod.EnvBasis = "bamboo"
	mod.Env = "bamboo"
	warn, err := mod.Write("super-secrets/tokens", tokenMap, coreConfig.Log)
	eUtils.LogErrorObject(coreConfig, err, false)
	eUtils.LogWarningsObject(coreConfig, warn, false)

	envStrings := SelectedEnvironment
	for _, e := range envStrings {
		mod.Env = e
		warn, err = il.UploadTemplateDirectory(coreConfig, mod, templatePath, nil)
		eUtils.LogErrorObject(coreConfig, err, false)
		eUtils.LogWarningsObject(coreConfig, warn, false)
	}

	err = v.EnableAppRole()
	eUtils.LogErrorObject(coreConfig, err, false)

	err = v.CreateNewRole("bamboo", &sys.NewRoleOptions{
		TokenTTL:    "10m",
		TokenMaxTTL: "15m",
		Policies:    []string{"bamboo"},
	})
	eUtils.LogErrorObject(coreConfig, err, false)

	roleID, _, err := v.GetRoleID("bamboo")
	eUtils.LogErrorObject(coreConfig, err, false)

	secretID, err := v.GetSecretID("bamboo")
	eUtils.LogErrorObject(coreConfig, err, false)

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
	s.InitConfig(coreConfig, targetEnv)

	res, err := s.APILogin(ctx, &pb.LoginReq{Username: req.Username, Password: req.Password, Environment: targetEnv})
	if err != nil {
		eUtils.LogErrorObject(coreConfig, err, false)
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
		s.VaultTokenPtr = &sToken
	} else {
		s.VaultTokenPtr = eUtils.EmptyStringRef()
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
	coreConfig := &core.CoreConfig{ExitOnFailure: false, Log: s.Log}

	mod, err := helperkv.NewModifier(false, s.VaultTokenPtr, s.VaultAddrPtr, "nonprod", nil, true, s.Log)
	if err != nil {
		eUtils.LogErrorObject(coreConfig, err, false)
		return &result, err
	}
	mod.Env = req.Environment

	authSuccess, name, err := s.authUser(coreConfig, mod, req.Username, req.Password)
	if err != nil {
		eUtils.LogErrorObject(coreConfig, err, false)
		return &result, err
	}

	token, errJwtGen := s.generateJWT(name, req.Environment+"/"+req.Username, mod)
	if errJwtGen != nil {
		eUtils.LogErrorObject(coreConfig, errJwtGen, false)
		return &result, err
	}
	result.AuthToken = token
	result.Success = authSuccess

	return &result, nil
}

// GetStatus requests version info and whether the vault has been initailized
func (s *Server) GetStatus(ctx context.Context, req *pb.NoParams) (*pb.VaultStatus, error) {
	v, err := sys.NewVault(false, s.VaultAddrPtr, "nonprod", true, false, false, s.Log)
	if v != nil {
		defer v.Close()
	}
	coreConfig := &core.CoreConfig{ExitOnFailure: false, Log: s.Log}
	if err != nil {
		eUtils.LogErrorObject(coreConfig, err, false)
		return nil, err
	}

	status, err := v.GetStatus()
	if err != nil {
		eUtils.LogErrorObject(coreConfig, err, false)
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
	v, err := sys.NewVault(false, s.VaultAddrPtr, "nonprod", false, false, false, s.Log)
	coreConfig := &core.CoreConfig{ExitOnFailure: false, Log: s.Log}
	if err != nil {
		eUtils.LogErrorObject(coreConfig, err, false)
		return nil, err
	}

	v.AddShard(req.UnsealKey)
	prog, need, sealed, err := v.Unseal()

	if err != nil {
		eUtils.LogErrorObject(coreConfig, err, false)
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
