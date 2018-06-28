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

//var logged string = "W0lOSVRdMjAxOC8wNi8yNiAwNzo0NzowNyBTdWNjZXNmdWxseSBjb25uZWN0ZWQgdG8gdmF1bHQgYXQgaHR0cDovLzEyNy4wLjAuMTo4MjAwCltJTklUXTIwMTgvMDYvMjYgMDc6NDc6MDcgQ3JlYXRlZCBlbmdpbmUgdGVtcGxhdGVzCltJTklUXTIwMTgvMDYvMjYgMDc6NDc6MDcgQ3JlYXRlZCBlbmdpbmUgdmFsdWVzCltJTklUXTIwMTgvMDYvMjYgMDc6NDc6MDcgQ3JlYXRlZCBlbmdpbmUgc3VwZXItc2VjcmV0cwpbSU5JVF0yMDE4LzA2LzI2IDA3OjQ3OjA3IENyZWF0ZWQgZW5naW5lIHZhbHVlLW1ldHJpY3MKW0lOSVRdMjAxOC8wNi8yNiAwNzo0NzowNyBDcmVhdGVkIGVuZ2luZSB2ZXJpZmljYXRpb24KW0lOSVRdMjAxOC8wNi8yNiAwNzo0NzowNyBLZXkgd2l0aCBubyB2YWx1ZSB3aWxsIG5vdCBiZSB3cml0dGVuOiA6IHZhbHVlcwpbSU5JVF0yMDE4LzA2LzI2IDA3OjQ3OjA3IEtleSB3aXRoIG5vIHZhbHVlIHdpbGwgbm90IGJlIHdyaXR0ZW46IDogdmFsdWUtbWV0cmljcwpbSU5JVF0yMDE4LzA2LzI2IDA3OjQ3OjA3IEtleSB3aXRoIG5vIHZhbHVlIHdpbGwgbm90IGJlIHdyaXR0ZW46IHRlbXBsYXRlczogU3BlY3RydW0KW0lOSVRdMjAxOC8wNi8yNiAwNzo0NzowNyBXcml0aW5nIHNlZWQgdmFsdWVzIHRvIHBhdGhzCltWRVJJRlldMjAxOC8wNi8yNiAwNzo0NzowNyBWZXJpZnlpbmcgU3BlY3RydW1EQiBhcyB0eXBlIGRiCltWRVJJRlldMjAxOC8wNi8yNiAwNzo0NzowOCAJdmVyaWZpZWQ6IHRydWUKW1ZFUklGWV0yMDE4LzA2LzI2IDA3OjQ3OjA4IFZlcmlmeWluZyBTZXJ2aWNlVGVjaERCIGFzIHR5cGUgZGIKW1ZFUklGWV0yMDE4LzA2LzI2IDA3OjQ3OjA4IAl2ZXJpZmllZDogdHJ1ZQpbVkVSSUZZXTIwMTgvMDYvMjYgMDc6NDc6MDggVmVyaWZ5aW5nIFNlbmRHcmlkIGFzIHR5cGUgU2VuZEdyaWRLZXkKW1ZFUklGWV0yMDE4LzA2LzI2IDA3OjQ3OjA4IAl2ZXJpZmllZDogZmFsc2UKW1ZFUklGWV0yMDE4LzA2LzI2IDA3OjQ3OjA4IFZlcmlmeWluZyBLZXlTdG9yZSBhcyB0eXBlIEtleVN0b3JlCltWRVJJRlldMjAxOC8wNi8yNiAwNzo0NzowOCAJdmVyaWZpZWQ6IGZhbHNlCltQT0xJQ1ldMjAxOC8wNi8yNiAwNzo0NzowOCBXcml0aW5nIHBvbGljaWVzIGZyb20gcG9saWN5X2ZpbGVzCltQT0xJQ1ldMjAxOC8wNi8yNiAwNzo0NzowOCAJRm91bmQgcG9saWN5IGZpbGU6IGNvbmZpZy5oY2wKW1BPTElDWV0yMDE4LzA2LzI2IDA3OjQ3OjA4IAlGb3VuZCBwb2xpY3kgZmlsZTogY29uZmlnX1FBLmhjbApbUE9MSUNZXTIwMTgvMDYvMjYgMDc6NDc6MDggCUZvdW5kIHBvbGljeSBmaWxlOiBjb25maWdfZGV2LmhjbApbUE9MSUNZXTIwMTgvMDYvMjYgMDc6NDc6MDggCUZvdW5kIHBvbGljeSBmaWxlOiBtZXRyaWNzLmhjbApbUE9MSUNZXTIwMTgvMDYvMjYgMDc6NDc6MDggCUZvdW5kIHBvbGljeSBmaWxlOiB0ZC5oY2wKW1BPTElDWV0yMDE4LzA2LzI2IDA3OjQ3OjA4IAlGb3VuZCBwb2xpY3kgZmlsZTogdGRfUUEuaGNsCltQT0xJQ1ldMjAxOC8wNi8yNiAwNzo0NzowOCAJRm91bmQgcG9saWN5IGZpbGU6IHRkX2Rldi5oY2wKW1RPS0VOXTIwMTgvMDYvMjYgMDc6NDc6MDggV3JpdGluZyB0b2tlbnMgZnJvbSB0b2tlbl9maWxlcwpbVE9LRU5dMjAxOC8wNi8yNiAwNzo0NzowOCAJRm91bmQgdG9rZW4gZmlsZTogY29uZmlnX3Rva2VuX2Rldi55bWwKW1RPS0VOXTIwMTgvMDYvMjYgMDc6NDc6MDggCUZvdW5kIHRva2VuIGZpbGU6IHRkX3Rva2VuX2Rldi55bWwK"

//InitVault Takes init request and inits/seeds vault with contained file data
func (s *Server) InitVault(ctx context.Context, req *pb.InitReq) (*pb.InitResp, error) {
	fmt.Println("Initing vault")
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

	tokens := il.UploadTokens(tokenPath, v, logger)

	mod, err := kv.NewModifier(s.VaultToken, s.VaultAddr, s.CertPath)
	utils.LogErrorObject(err, logger)
	fmt.Printf("Adding user %s: %s\n", req.Username, req.Password)
	warns, err := mod.Write("cubbyhole/credentials", map[string]interface{}{
		req.Username: req.Password,
	})
	utils.LogWarningsObject(warns, logger)
	utils.LogErrorObject(err, logger)
	return &pb.InitResp{
		Success: true,
		Logfile: b64.StdEncoding.EncodeToString(logBuffer.Bytes()),
		Tokens:  tokens,
	}, nil
}

// func (s *Server) InitVault(ctx context.Context, req *pb.InitReq) (*pb.InitResp, error) {
// 	return &pb.InitResp{
// 		Success: true,
// 		Logfile: logged,
// 	}, nil
// }

//APILogin Verifies the user's login with the cubbyhole
func (s *Server) APILogin(ctx context.Context, req *pb.LoginReq) (*pb.LoginResp, error) {
	fmt.Printf("Req: %v\n", req)
	mod, err := kv.NewModifier(s.VaultToken, s.VaultAddr, s.CertPath)
	if err != nil {
		return nil, err
	}
	pass, err := mod.ReadValue("cubbyhole/credentials", req.Username)
	if err != nil {
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
		return nil, err
	}

	status, err := v.GetStatus()
	if err != nil {
		return nil, err
	}
	return &pb.VaultStatus{
		Version:     status["version"].(string),
		Initialized: status["initialized"].(bool),
		Sealed:      status["sealed"].(bool),
	}, err
}
