package server

import (
	"bitbucket.org/dexterchaney/whoville/utils"
	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
	sys "bitbucket.org/dexterchaney/whoville/vault-helper/system"
	il "bitbucket.org/dexterchaney/whoville/vault-init/initlib"
	pb "bitbucket.org/dexterchaney/whoville/webapi/rpc/apinator"
	"bytes"
	"context"
	b64 "encoding/base64"
	"fmt"
	"log"
)

const tokenPath string = "token_files"
const policyPath string = "policy_files"

//InitVault Takes init request and inits/seeds vault with contained file data
func (s *Server) InitVault(ctx context.Context, req *pb.InitReq) (*pb.InitResp, error) {
	fmt.Println("Initing vault")
	logBuffer := new(bytes.Buffer)
	logger := log.New(logBuffer, "[INIT]", log.LstdFlags)

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
		utils.LogErrorObject(err, s.Log, false)
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
	fmt.Printf("Root token: %s\n", s.VaultToken)
	//check error returned by unseal
	v.Unseal()
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
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
			utils.LogErrorObject(err, s.Log, false)
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

	mod, err := kv.NewModifier(s.VaultToken, s.VaultAddr, s.CertPath)
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		utils.LogErrorObject(err, logger, false)
		return &pb.InitResp{
			Success: false,
			Logfile: b64.StdEncoding.EncodeToString(logBuffer.Bytes()),
			Tokens:  nil,
		}, err
	}
	fmt.Printf("Adding user %s: %s\n", req.Username, req.Password)
	warns, err := mod.Write("cubbyhole/credentials", map[string]interface{}{
		req.Username: req.Password,
	})
	utils.LogWarningsObject(warns, logger, false)
	utils.LogWarningsObject(warns, s.Log, false)
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		utils.LogErrorObject(err, logger, false)
		return &pb.InitResp{
			Success: false,
			Logfile: b64.StdEncoding.EncodeToString(logBuffer.Bytes()),
			Tokens:  nil,
		}, err
	}
	return &pb.InitResp{
		Success: true,
		Logfile: b64.StdEncoding.EncodeToString(logBuffer.Bytes()),
		Tokens:  tokens,
	}, nil
}

//APILogin Verifies the user's login with the cubbyhole
func (s *Server) APILogin(ctx context.Context, req *pb.LoginReq) (*pb.LoginResp, error) {
	fmt.Printf("Req: %v\n", req)
	mod, err := kv.NewModifier(s.VaultToken, s.VaultAddr, s.CertPath)
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}
	pass, err := mod.ReadValue("cubbyhole/credentials", req.Username)
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}

	success := pass == req.Password
	fmt.Printf("%s %s == %v\n", pass, req.Password, success)
	if pass != "" && success {
		// Generate token
		return &pb.LoginResp{
			Success:   true,
			AuthToken: "TODO",
		}, nil

	}
	return &pb.LoginResp{
		Success:   false,
		AuthToken: "LOGIN_FAILURE",
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
	return &pb.UnsealResp{
		Sealed:   sealed,
		Progress: int32(prog),
		Needed:   int32(need),
	}, nil
}
