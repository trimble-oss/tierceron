package trcshauth

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"embed"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/trimble-oss/tierceron-hat/cap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

//go:embed tls/mashup.crt
var MashupCert embed.FS

//go:embed tls/mashup.key
var MashupKey embed.FS

var mashupCertPool *x509.CertPool

func init() {
	rand.Seed(time.Now().UnixNano())
	mashupCertBytes, err := MashupCert.ReadFile("tls/mashup.crt")
	if err != nil {
		fmt.Println("Cert read failure.")
		return
	}

	mashupBlock, _ := pem.Decode([]byte(mashupCertBytes))

	mashupClientCert, parseErr := x509.ParseCertificate(mashupBlock.Bytes)
	if parseErr != nil {
		fmt.Println("Cert parse read failure.")
		return
	}
	mashupCertPool = x509.NewCertPool()
	mashupCertPool.AddCert(mashupClientCert)
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func randomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func PenseQuery(pense string) (string, error) {
	penseCode := randomString(7 + rand.Intn(7))
	penseArray := sha256.Sum256([]byte(penseCode))
	penseSum := hex.EncodeToString(penseArray[:])

	capWriteErr := cap.TapWriter(penseSum)
	if capWriteErr != nil {
		return "", errors.Join(errors.New("Tap writer error"), capWriteErr)
	}

	conn, err := grpc.Dial("127.0.0.1:12384", grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{ServerName: "", RootCAs: mashupCertPool, InsecureSkipVerify: true})))
	if err != nil {
		return "", err
	}
	defer conn.Close()
	c := cap.NewCapClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	r, penseErr := c.Pense(ctx, &cap.PenseRequest{Pense: penseCode, PenseIndex: pense})
	if penseErr != nil {
		return "", errors.Join(errors.New("Pense error"), penseErr)
	}

	return r.GetPense(), nil
}
