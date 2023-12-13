package server

import (
	"context"
	"fmt"
	"time"

	jwt "github.com/golang-jwt/jwt"

	eUtils "github.com/trimble-oss/tierceron/utils"
	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"
	sys "github.com/trimble-oss/tierceron/vaulthelper/system"
	pb "github.com/trimble-oss/tierceron/webapi/rpc/apinator"
)

func (s *Server) generateJWT(user string, id string, mod *helperkv.Modifier) (string, error) {
	tokenSecret := s.TrcAPITokenSecret
	currentTime := time.Now().Unix()
	expTime := currentTime + 24*60*60
	config := &eUtils.DriverConfig{ExitOnFailure: false, Log: s.Log}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  id,
		"name": user,
		"iss":  "Viewpoint, Inc.",
		"aud":  "Viewpoint Vault WebAPI",
		"iat":  currentTime,
		"exp":  expTime,
	})

	// Upload token information to vault
	if mod != nil {
		defer func() {
			tokenData := map[string]interface{}{
				"ID":      id,
				"Issued":  currentTime,
				"Expires": expTime,
			}
			warn, err := mod.Write("apiLogins/"+user, tokenData, config.Log)
			eUtils.LogWarningsObject(config, warn, false)
			eUtils.LogErrorObject(config, err, false)
		}()
	}

	return token.SignedString(tokenSecret)
}

// GetVaultTokens takes app role credentials and attempts to fetch names tokens from the vault
func (s *Server) GetVaultTokens(ctx context.Context, req *pb.TokensReq) (*pb.TokensResp, error) {
	// Create 2 vault connections, one for checking/rolling tokens, the other for accessing the AWS user cubbyhole
	v, err := sys.NewVault(false, s.VaultAddr, "nonprod", false, false, false, s.Log)
	config := &eUtils.DriverConfig{ExitOnFailure: false, Log: s.Log}
	if v != nil {
		defer v.Close()
	}

	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		return nil, err
	}

	v.SetToken(s.VaultToken)

	if len(req.AppRoleID) == 0 || len(req.AppRoleSecretID) == 0 {
		return nil, fmt.Errorf("need both role ID and secret ID to login through app role")
	}

	arToken, err := v.AppRoleLogin(req.AppRoleID, req.AppRoleSecretID)
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		return nil, err
	}

	// Modifier to access token values granted to bamboo
	mod, err := helperkv.NewModifier(false, arToken, s.VaultAddr, "nonprod", nil, true, s.Log)
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		return nil, err
	}
	mod.RawEnv = "bamboo"
	mod.Env = "bamboo"

	data, err := mod.ReadData("super-secrets/tokens")
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		return nil, err
	}

	reqTokens := make(map[string]bool, len(req.Tokens))
	for _, k := range req.Tokens {
		reqTokens[k] = true
	}

	tokens := []*pb.TokensResp_Token{}

	for k, v := range data {
		if token, ok := v.(string); ok {
			if len(reqTokens) == 0 || reqTokens[k] {
				tokens = append(tokens, &pb.TokensResp_Token{
					Name:  k,
					Value: token,
				})
			}
		} else {
			eUtils.LogWarningsObject(config, []string{fmt.Sprintf("Failed to convert token %s to string", k)}, false)
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

// RollTokens checks the validity of tokens in super-secrets/bamboo/tokens and rerolls them
func (s *Server) RollTokens(ctx context.Context, req *pb.NoParams) (*pb.NoParams, error) {
	return &pb.NoParams{}, nil
}
