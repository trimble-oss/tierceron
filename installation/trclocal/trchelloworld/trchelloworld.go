package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"gopkg.in/yaml.v2"

	pb "github.com/trimble-oss/tierceron/installation/trclocal/trchelloworld/hellosdk" // Update package path as needed
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type server struct {
	pb.UnimplementedGreeterServer
}

func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	log.Printf("Received: %v", in.GetName())
	return &pb.HelloReply{Message: "Hello " + in.GetName()}, nil
}

func main() {
	logFilePtr := flag.String("log", "./trchelloworld.log", "Output path for log file")
	//tokenPtr := flag.String("token", "", "Vault access Token")
	flag.Parse()

	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		fmt.Printf("Error opening log file: %v\n", err)
		os.Exit(-1)
	}
	logger := log.New(f, "[trchelloworld]", log.LstdFlags)

	data, err := os.ReadFile("config.yml")
	if err != nil {
		logger.Println("Error reading YAML file:", err)
		os.Exit(-1)
	}

	// Create an empty map for the YAML data
	var config map[string]any

	// Unmarshal the YAML data into the map
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		logger.Println("Error unmarshaling YAML:", err)
		os.Exit(-1)
	}

	if port, ok := config["grpc_server_port"].(int); ok {

		useSsl := true
		helloCertBytes, err := os.ReadFile("./hello.crt")
		if err != nil {
			log.Printf("Couldn't load cert: %v", err)
			useSsl = false
		}

		helloKeyBytes, err := os.ReadFile("./hellokey.key")
		if err != nil {
			log.Printf("Couldn't load key: %v", err)
			useSsl = false
		}

		cert, err := tls.X509KeyPair(helloCertBytes, helloKeyBytes)
		if err != nil {
			log.Printf("Couldn't construct key pair: %v", err)
			useSsl = false
		}
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))

		if useSsl {
			creds := credentials.NewServerTLSFromCert(&cert)
			grpcServer := grpc.NewServer(grpc.Creds(creds))

			pb.RegisterGreeterServer(grpcServer, &server{})
			log.Printf("server listening at %v", lis.Addr())
			if err := grpcServer.Serve(lis); err != nil {
				log.Fatalf("failed to serve: %v", err)
			}
		} else {
			if err != nil {
				log.Fatalf("failed to listen: %v", err)
			}
			grpcServer := grpc.NewServer()
			pb.RegisterGreeterServer(grpcServer, &server{})
			log.Printf("server listening at %v", lis.Addr())
			if err := grpcServer.Serve(lis); err != nil {
				log.Fatalf("failed to serve: %v", err)
			}
		}

	}
}
