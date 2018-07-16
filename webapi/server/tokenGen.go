package server

import (
	b32 "encoding/base32"
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
