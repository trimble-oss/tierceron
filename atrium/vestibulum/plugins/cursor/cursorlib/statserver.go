package cursorlib

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	certutil "github.com/trimble-oss/tierceron/pkg/core/util/cert"

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

var GlobalStats *cmap.ConcurrentMap[string, string]

var globalToken *string

var m sync.Mutex

type statServiceServer struct {
	pb.UnimplementedStatServiceServer
}

func (s *statServiceServer) GetStats(ctx context.Context, req *pb.GetStatRequest) (*pb.GetStatResponse, error) {
	token := req.GetToken()
	if !eUtils.RefEquals(globalToken, token) {
		logger.Println("Unauthorized attempt to access statistics.")
		return &pb.GetStatResponse{
			Results: "",
		}, errors.New("unauthorized to access statistic server")
	}
	key := req.GetKey()
	value, ok := (*GlobalStats).Get(key)
	if !ok {
		return &pb.GetStatResponse{
			Results: "",
		}, nil
	}
	return &pb.GetStatResponse{
		Results: value,
	}, nil
}

func (s *statServiceServer) SetStats(ctx context.Context, req *pb.UpdateStatRequest) (*pb.UpdateStatResponse, error) {
	token := req.GetToken()
	if !eUtils.RefEquals(globalToken, token) {
		logger.Println("Unauthorized attempt to set statistics.")
		return &pb.UpdateStatResponse{
			Success: false,
		}, errors.New("unauthorized to set statistics in server")
	}
	key := req.GetKey()
	value := req.GetValue()
	(*GlobalStats).Set(key, value)
	return &pb.UpdateStatResponse{
		Success: true,
	}, nil
}

func (s *statServiceServer) IncrementStats(ctx context.Context, req *pb.UpdateStatRequest) (*pb.UpdateStatResponse, error) {
	token := req.GetToken()
	if !eUtils.RefEquals(globalToken, token) {
		logger.Println("Unauthorized attempt to set statistics.")
		return &pb.UpdateStatResponse{
			Success: false,
		}, errors.New("unauthorized to set statistics in server")
	}
	m.Lock()
	defer m.Unlock()
	data_type := req.GetDatatype()
	key := req.GetKey()
	value := req.GetValue()
	prev_value, ok := (*GlobalStats).Get(key)
	var err error
	var total_value string
	if data_type == "int" {
		total := 0
		if ok {
			total, err = strconv.Atoi(prev_value)
			if err != nil {
				logger.Printf("error converting stats for incrementing int value: %v\n", err)
				return &pb.UpdateStatResponse{
					Success: false,
				}, errors.New("error converting stats for incrementing int value")
			}
		}
		toAdd := 0
		if v, err := strconv.Atoi(value); err == nil {
			toAdd = v
		} else {
			logger.Println("Different type of value passed in than specified.")
			return &pb.UpdateStatResponse{
				Success: false,
			}, errors.New("different type of value passed in than specified")
		}
		total = total + toAdd
		total_value = strconv.Itoa(total)
	} else if data_type == "float64" {
		var prev float64 = 0
		if ok {
			prev, err = strconv.ParseFloat(prev_value, 64)
			if err != nil {
				logger.Printf("error converting stats for incrementing float value: %v", err)
				return &pb.UpdateStatResponse{
					Success: false,
				}, errors.New("error converting stats for incrementing float value")
			}
		}
		var toAdd float64 = 0
		if t, err := strconv.ParseFloat(value, 64); err == nil {
			toAdd = t
		} else {
			logger.Println("Different type of value passed in than specified.")
			return &pb.UpdateStatResponse{
				Success: false,
			}, errors.New("different type of value passed in than specified")
		}
		total_value = fmt.Sprintf("%v", prev+toAdd)
	} else {
		logger.Println("Unsupported data type for statistics server.")
		return &pb.UpdateStatResponse{
			Success: false,
		}, errors.New("unsupported data type for statistics server")
	}

	(*GlobalStats).Set(key, total_value)
	return &pb.UpdateStatResponse{
		Success: true,
	}, nil
}

func resetLongestConverting(key string) error {
	for {
		currentTime := time.Now()
		nextHour := currentTime.Truncate(time.Hour).Add(time.Hour)
		durationUntilNextHour := nextHour.Sub(currentTime)
		time.Sleep(durationUntilNextHour)
		var t float64 = 0
		(*GlobalStats).Set(key, fmt.Sprintf("%f", t))
	}
}

func (s *statServiceServer) UpdateMaxStats(ctx context.Context, req *pb.UpdateStatRequest) (*pb.UpdateStatResponse, error) {
	token := req.GetToken()
	if !eUtils.RefEquals(globalToken, token) {
		logger.Println("Unauthorized attempt to update max statistics.")
		return &pb.UpdateStatResponse{
			Success: false,
		}, errors.New("unauthorized to update max statistics in server")
	}
	m.Lock()
	defer m.Unlock()
	data_type := req.GetDatatype()
	key := req.GetKey()
	value := req.GetValue()
	prev_value, ok := (*GlobalStats).Get(key)
	if !ok && strings.HasPrefix(key, "LONGEST_PDF_CONVERTING") {
		go resetLongestConverting(key)
	}
	var err error
	var max_value string
	reset := false
	if data_type == "int" {
		old_val := 0
		if ok {
			old_val, err = strconv.Atoi(prev_value)
			if err != nil {
				logger.Printf("error converting stats for updating max int value: %v", err)
				return &pb.UpdateStatResponse{
					Success: false,
				}, errors.New("error converting stats for updating max int value")
			}
		}
		new_val := 0
		if v, err := strconv.Atoi(value); err == nil {
			new_val = v
		} else {
			logger.Println("Different type of value passed in than specified.")
			return &pb.UpdateStatResponse{
				Success: false,
			}, errors.New("different type of value passed in than specified")
		}
		if new_val > old_val {
			reset = true
			old_val = new_val
		}
		max_value = strconv.Itoa(old_val)
	} else if data_type == "float64" {
		var prev float64 = 0
		if ok {
			prev, err = strconv.ParseFloat(prev_value, 64)
			if err != nil {
				logger.Printf("error converting stats for updating max float value: %v", err)
				return &pb.UpdateStatResponse{
					Success: false,
				}, errors.New("error converting stats for updating max float value")
			}
		}
		var new_val float64 = 0
		if t, err := strconv.ParseFloat(value, 64); err == nil {
			new_val = t
		} else {
			logger.Println("Different type of value passed in than specified.")
			return &pb.UpdateStatResponse{
				Success: false,
			}, errors.New("different type of value passed in than specified")
		}
		if new_val > prev {
			reset = true
			prev = new_val
		}
		max_value = fmt.Sprintf("%v", prev)
	} else {
		logger.Println("Unsupported data type for updating max in statistics server.")
		return &pb.UpdateStatResponse{
			Success: false,
		}, errors.New("unsupported data type for updating max in statistics server")
	}

	(*GlobalStats).Set(key, max_value)
	return &pb.UpdateStatResponse{
		Success: reset,
	}, nil
}

func InitStats() {
	ccmap := cmap.New[string]()
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
	trcshDriverConfig.DriverConfig, goMod, vault, err = eUtils.InitVaultModForPlugin(pluginConfig,
		trcshDriverConfig.DriverConfig.CoreConfig.TokenCache,
		"config_token_pluginany", trcshDriverConfig.DriverConfig.CoreConfig.Log)
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

	if t, ok := certifyMap["trcstatstoken"].(string); ok {
		globalToken = &t
	} else {
		logger.Printf("No valid token found for trcstats server.\n")
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
		statCert, err := certutil.LoadCertComponent(trcshDriverConfig.DriverConfig,
			goMod,
			tccore.TRCSHHIVEK_CERT)

		if err != nil {
			log.Printf("Couldn't load cert: %v", err)
			return err
		}
		statKey, err := certutil.LoadCertComponent(trcshDriverConfig.DriverConfig,
			goMod,
			tccore.TRCSHHIVEK_KEY)

		if err != nil {
			log.Printf("Couldn't load key: %v", err)
			return err
		}

		// Initialize the stats.
		InitStats()

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
