package server

import (
	"bitbucket.org/dexterchaney/whoville/utils"
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
	logBuffer := new(bytes.Buffer)
	logger := log.New(logBuffer, "[INIT]", log.LstdFlags)
	v, err := sys.NewVault(s.VaultAddr, s.CertPath)
	utils.LogErrorObject(err, logger)

	// Init and unseal vault
	keyToken, err := v.InitVault(1, 1)
	if err != nil {
		return nil, err
	}
	v.SetToken(keyToken.Token)
	v.SetShards(keyToken.Keys)
	s.VaultToken = keyToken.Token
	fmt.Printf("Root token: %s\n", s.VaultToken)
	//check error returned by unseal
	v.Unseal()
	if err != nil {
		return nil, err
	}

	logger.Printf("Succesfully connected to vault at %s\n", s.VaultAddr)

	// Create engines
	il.CreateEngines(v, logger)

	for _, seed := range req.Files {
		fBytes, err := b64.StdEncoding.DecodeString(seed.Data)
		if err != nil {
			return nil, err
		}
		il.SeedVaultFromData(fBytes, s.VaultAddr, s.VaultToken, seed.Env, s.CertPath, logger)
	}

	il.UploadPolicies(policyPath, v, logger)

	il.UploadTokens(tokenPath, v, logger)

	logger.SetPrefix("[INIT]")
	return &pb.InitResp{
		Success: true,
		Logfile: b64.StdEncoding.EncodeToString(logBuffer.Bytes()),
	}, nil
}
