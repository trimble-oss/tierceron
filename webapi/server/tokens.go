package server

import (
	"bitbucket.org/dexterchaney/whoville/utils"
	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
	sys "bitbucket.org/dexterchaney/whoville/vault-helper/system"
	pb "bitbucket.org/dexterchaney/whoville/webapi/rpc/apinator"
	"context"
	b32 "encoding/base32"
	"fmt"
	jwt "github.com/dgrijalva/jwt-go"
	"time"
)

func generateJWT(user string) (string, error) {
	tokenSecret := []byte("V2hvVmlsbDMhVjR1N1Q9UHIwajNjVA==")
	currentTime := time.Now().Unix()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  b32.StdEncoding.EncodeToString([]byte(user)),
		"name": user,
		"iss":  "Viewpoint, Inc.",
		"aud":  "Viewpoint Vault WebAPI",
		"iat":  currentTime,
		"exp":  currentTime + 24*60*60,
	})

	return token.SignedString(tokenSecret)
}

// GetVaultTokens takes app role credentials and attempts to fetch names tokens from the vault
func (s *Server) GetVaultTokens(ctx context.Context, req *pb.TokensReq) (*pb.TokensResp, error) {
	// Create 2 vault connections, one for checking/rolling tokens, the other for accessing the AWS user cubbyhole
	v, err := sys.NewVault(s.VaultAddr, s.CertPath)
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}

	v.SetToken(s.VaultToken)

	if len(req.AppRoleID) == 0 || len(req.AppRoleSecretID) == 0 {
		return nil, fmt.Errorf("Need both role ID and secret ID to login through app role")
	}

	arToken, err := v.AppRoleLogin(req.AppRoleID, req.AppRoleSecretID)
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}

	// Modifier to access token values granted to bamboo
	mod, err := kv.NewModifier(arToken, s.VaultAddr, s.CertPath)
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}
	mod.Env = "bamboo"

	data, err := mod.ReadData("super-secrets/tokens")
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}

	tokens := []*pb.TokensResp_Token{}

	for k, v := range data {
		if token, ok := v.(string); ok {
			tokens = append(tokens, &pb.TokensResp_Token{
				Name:  k,
				Value: token,
			})
		}
	}
	// AWS
	// Get tokens from cubbyhole

	// TOKEN ROLLER
	// Check state of tokens, reroll tokens within 1h of expiration

	// AWS
	// Store newly rolled tokens

	return &pb.TokensResp{Tokens: tokens}, nil
}

func (s *Server) RollTokens(ctx context.Context, req *pb.NoParams) (*pb.NoParams, error) {
	return &pb.NoParams{}, nil
}
