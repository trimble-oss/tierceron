package cursorlib

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"

	"github.com/trimble-oss/tierceron/buildopts/cursoropts"
	"github.com/trimble-oss/tierceron/pkg/capauth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	cmap "github.com/orcaman/concurrent-map/v2"

	pb "github.com/trimble-oss/tierceron-core/v2/statsdk"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"

	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
	sys "github.com/trimble-oss/tierceron/pkg/vaulthelper/system"
)

var GlobalStats *cmap.ConcurrentMap[string, interface{}]

type statServiceServer struct {
	pb.UnimplementedStatServiceServer
}

func (s *statServiceServer) GetStats(ctx context.Context, req *pb.GetStatRequest) (*pb.GetStatResponse, error) {
	key := req.GetKey()
	value, ok := (*GlobalStats).Get(key)
	formatted_val := fmt.Sprintf("%v", value)
	if !ok {
		return &pb.GetStatResponse{
			Results:  "",
			DataType: "",
		}, nil
	}
	data_type := fmt.Sprintf("%T", value)
	return &pb.GetStatResponse{
		Results:  formatted_val,
		DataType: data_type,
	}, nil
}

func (s *statServiceServer) SetStats(ctx context.Context, req *pb.SetStatRequest) (*pb.SetStatResponse, error) {
	key := req.GetKey()
	value := req.GetValue()
	data_type := req.GetDataType()
	if data_type == "int" {
		if data, ok := strconv.Atoi(value); ok != nil {
			(*GlobalStats).Set(key, data)
			return &pb.SetStatResponse{
				Success: true,
			}, nil
		} else {
			return &pb.SetStatResponse{
				Success: false,
			}, errors.New("incorrect data type and value specified")
		}
	} else if data_type == "string" {
		(*GlobalStats).Set(key, value)
		return &pb.SetStatResponse{
			Success: true,
		}, nil
	} else if data_type == "float64" {
		if data, ok := strconv.ParseFloat(value, 64); ok != nil {
			(*GlobalStats).Set(key, data)
			return &pb.SetStatResponse{
				Success: true,
			}, nil
		} else {
			return &pb.SetStatResponse{
				Success: false,
			}, errors.New("incorrect data type and value specified")
		}
	}
	return &pb.SetStatResponse{
		Success: false,
	}, errors.New("unexpected data type specified")
}

func InitStats() {
	ccmap := cmap.New[interface{}]()
	GlobalStats = &ccmap
}

func InitServer(port int, certBytes []byte, keyBytes []byte) (net.Listener, *grpc.Server, error) {
	var err error

	cert, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		log.Printf("Couldn't construct key pair: %v\n", err) //Should this just return instead?? - no panic
	}
	creds := credentials.NewServerTLSFromCert(&cert)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		fmt.Println("Failed to listen:", err)
		return nil, nil, err
	}

	grpcServer := grpc.NewServer(grpc.Creds(creds))

	return lis, grpcServer, nil
}

func StatServerInit(trcshDriverConfig *capauth.TrcshDriverConfig, pluginConfig map[string]interface{}) error {
	var goMod *helperkv.Modifier
	var vault *sys.Vault
	var err error

	//Grabbing configs
	tempAddr := pluginConfig["vaddress"]
	tempTokenPtr := pluginConfig["tokenptr"]
	if cAddr, cAddressOk := pluginConfig["caddress"].(string); cAddressOk && len(cAddr) > 0 {
		pluginConfig["vaddress"] = cAddr
	} else {
		eUtils.LogWarningMessage(trcshDriverConfig.DriverConfig.CoreConfig, "Unexpectedly caddress not available", false)
	}
	if cTokenPtr, cTokOk := pluginConfig["ctokenptr"].(*string); cTokOk && eUtils.RefLength(cTokenPtr) > 0 {
		pluginConfig["tokenptr"] = cTokenPtr
	}

	if tokenPtr, tokPtrOk := pluginConfig["tokenptr"].(*string); tokPtrOk && eUtils.RefLength(tokenPtr) < 5 {
		eUtils.LogWarningMessage(trcshDriverConfig.DriverConfig.CoreConfig, "WARNING: Unexpectedly token not available", false)
	}
	trcshDriverConfig.DriverConfig, goMod, vault, err = eUtils.InitVaultModForPlugin(pluginConfig, "", trcshDriverConfig.DriverConfig.CoreConfig.Log)
	if vault != nil {
		defer vault.Close()
	}

	if goMod != nil {
		defer goMod.Release()
	}
	pluginConfig["vaddress"] = tempAddr
	pluginConfig["tokenptr"] = tempTokenPtr

	if err != nil {
		eUtils.LogErrorMessage(trcshDriverConfig.DriverConfig.CoreConfig, "Could not access vault.  Failure to start.", true)
		return err
	}

	pluginName := cursoropts.BuildOptions.GetPluginName(true)
	logger.Printf("Loading data for %s\n", pluginName)

	certifyMap, err := goMod.ReadData(fmt.Sprintf("super-secrets/Index/TrcVault/trcplugin/%s/Certify", pluginName))
	if err != nil {
		logger.Printf("Validating Certification failure for %s %s\n", pluginName, err)
		return err
	}

	if portInterface, ok := certifyMap["trcstatsport"]; ok {
		var trcstatsport int
		if port, ok := portInterface.(int); ok {
			trcstatsport = port
		} else {
			var err error
			trcstatsport, err = strconv.Atoi(portInterface.(string))
			if err != nil {
				logger.Printf("Failed to process server port: %v", err)
				return err
			}
		}
		fmt.Printf("Server listening on :%d\n", trcstatsport)
		//load certs differently in future....
		statCert, err := os.ReadFile("./local_config/stat.crt") //need full path if debugging locally
		if err != nil {
			log.Printf("Couldn't load cert: %v", err)
		}

		statKey, err := os.ReadFile("./local_config/statkey.key")
		if err != nil {
			log.Printf("Couldn't load key: %v", err)
		}
		lis, gServer, err := InitServer(trcstatsport,
			statCert,
			statKey)
		if err != nil {
			logger.Printf("Failed to start server: %v", err)
			return err
		}
		logger.Println("Starting server")

		grpcServer := gServer
		grpc_health_v1.RegisterHealthServer(grpcServer, health.NewServer())
		pb.RegisterStatServiceServer(grpcServer, &statServiceServer{})
		// reflection.Register(grpcServer)
		// addr := lis.Addr().String()
		logger.Printf("server listening at %v", lis.Addr())
		go func(l net.Listener, logger *log.Logger) {
			if err := grpcServer.Serve(l); err != nil {
				logger.Println("Failed to serve:", err)
				return
			}
		}(lis, logger)
	}
	return nil
}
