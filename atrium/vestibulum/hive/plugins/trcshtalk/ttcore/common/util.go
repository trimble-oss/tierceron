package common

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"golang.org/x/exp/rand"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// GenMsgID produces a unique message id (broadcast aware).
func GenMsgID(env, region string, broadcast bool) string {
	rand.Seed(uint64(time.Now().UnixNano()))
	n := rand.Intn(10000000)
	r := region
	if r == "west" {
		r = ""
	}
	if broadcast {
		return fmt.Sprintf("%s:%s:b:%d", env, r, n)
	}
	return fmt.Sprintf("%s:%s:%d", env, r, n)
}

// InitServer creates a TLS gRPC server listener.
func InitServer(port int, certBytes, keyBytes []byte) (net.Listener, *grpc.Server, error) {
	cert, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("key pair: %w", err)
	}
	creds := credentials.NewServerTLSFromCert(&cert)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, nil, err
	}
	g := grpc.NewServer(grpc.Creds(creds))
	return lis, g, nil
}

// BuildClientConn dials a TLS gRPC server using a provided server cert (PEM).
func BuildClientConn(serverName string, port int, certPEM []byte) (*grpc.ClientConn, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, errors.New("invalid cert pem")
	}
	xc, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	pool.AddCert(xc)
	return grpc.Dial(fmt.Sprintf("%s:%d", serverName, port), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{ServerName: serverName, RootCAs: pool, InsecureSkipVerify: true})))
}

// Adapter func types to let ttcore provide its context-specific logging / channels without circular deps.
type DFStat interface {
	GetDeliverStatCtx() (context.Context, interface{}, error)
	FinishStatistic(string, string, string, *log.Logger, bool, interface{})
	UpdateDataFlowStatistic(string, string, string, string, int, func(string, error))
}

type ErrorReporter func(error)
