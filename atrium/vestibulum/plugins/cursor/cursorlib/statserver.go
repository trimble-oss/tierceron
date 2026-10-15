package cursorlib

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	certutil "github.com/trimble-oss/tierceron/pkg/core/util/cert"
	"github.com/trimble-oss/tierceron/pkg/utils/config"

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

// convertingTimers tracks hourly reset timers per LONGEST_PDF_CONVERTING key to prevent goroutine leaks
var convertingTimers *cmap.ConcurrentMap[string, *time.Timer]

var (
	statsServerMu       sync.Mutex
	statsServer         *grpc.Server
	statsCertReloadOnce sync.Once
)

const statsCertReloadInterval = 30 * time.Second

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

// scheduleResetLongestConverting schedules a single hourly reset timer for the given key.
// It cancels any existing timer for that key to prevent goroutine accumulation.
func scheduleResetLongestConverting(key string) {
	// Cancel existing timer for this key if it exists
	if existingTimer, exists := convertingTimers.Get(key); exists {
		existingTimer.Stop()
	}

	// Calculate duration until next hour
	currentTime := time.Now()
	nextHour := currentTime.Truncate(time.Hour).Add(time.Hour)
	durationUntilNextHour := nextHour.Sub(currentTime)

	// Create new timer that fires once at next hour
	newTimer := time.AfterFunc(durationUntilNextHour, func() {
		var t float64 = 0
		(*GlobalStats).Set(key, fmt.Sprintf("%f", t))

		// Recursively schedule the next hourly reset
		scheduleResetLongestConverting(key)
	})

	convertingTimers.Set(key, newTimer)
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
		scheduleResetLongestConverting(key)
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
	convertingTimersMap := cmap.New[*time.Timer]()
	convertingTimers = &convertingTimersMap
}

func InitServer(port int, certBytes []byte, keyBytes []byte) (net.Listener, *grpc.Server, error) {
	cert, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't construct stats server key pair: %w", err)
	}
	creds := credentials.NewServerTLSFromCert(&cert)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to listen:", err)
		return nil, nil, err
	}

	grpcServer := grpc.NewServer(grpc.Creds(creds))

	return lis, grpcServer, nil
}

func startStatsServer(port int, certBytes, keyBytes []byte) error {
	lis, server, err := InitServer(port, certBytes, keyBytes)
	if err != nil {
		return err
	}
	grpc_health_v1.RegisterHealthServer(server, health.NewServer())
	pb.RegisterStatServiceServer(server, &statServiceServer{})
	statsServer = server
	go func() {
		if err := server.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			logger.Printf("Stats gRPC service stopped with error: %v", err)
		}
	}()
	logger.Printf("Stats gRPC service listening at %v", lis.Addr())
	return nil
}

func restartStatsServer(port int, certBytes, keyBytes, previousCert, previousKey []byte) error {
	statsServerMu.Lock()
	defer statsServerMu.Unlock()

	logger.Printf("Stats gRPC service restarting to load updated certificates on port %d", port)
	if statsServer != nil {
		statsServer.Stop()
		statsServer = nil
	}
	if err := startStatsServer(port, certBytes, keyBytes); err != nil {
		logger.Printf("Stats gRPC service restart failed: %v", err)
		if restoreErr := startStatsServer(port, previousCert, previousKey); restoreErr != nil {
			logger.Printf("Stats gRPC service could not restore its previous certificate: %v", restoreErr)
			return fmt.Errorf("restart failed: %v; restore failed: %w", err, restoreErr)
		}
		logger.Println("Stats gRPC service restarted with its previous certificate")
		return err
	}
	logger.Printf("Stats gRPC service restarted successfully on port %d", port)
	return nil
}

func startStatsCertReloader(driverConfig *config.DriverConfig, pluginConfig map[string]any, port int) {
	if driverConfig == nil || driverConfig.CoreConfig == nil || driverConfig.CoreConfig.TokenCache == nil {
		logger.Println("Stats certificate reloader not started: Vault configuration is unavailable")
		return
	}
	statsCertReloadOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(statsCertReloadInterval)
			defer ticker.Stop()
			var mod *helperkv.Modifier
			var vault *sys.Vault
			defer func() {
				if mod != nil {
					mod.Release()
				}
				if vault != nil {
					vault.Close()
				}
			}()
			for range ticker.C {
				if mod == nil {
					currentTokenName := "config_token_pluginany"
					vaultConfig := map[string]any{
						"vaddress": pluginConfig["vaddress"],
						"tokenptr": pluginConfig["tokenptr"],
						"env":      driverConfig.CoreConfig.EnvBasis,
					}
					var err error
					_, mod, vault, err = eUtils.InitVaultModForPlugin(vaultConfig, driverConfig.CoreConfig.TokenCache, currentTokenName, driverConfig.CoreConfig.Log)
					if err != nil {
						logger.Printf("Stats certificate reloader could not connect to Vault: %v", err)
						if mod != nil {
							mod.Release()
						}
						mod = nil
						if vault != nil {
							vault.Close()
							vault = nil
						}
						continue
					}
					if mod == nil {
						logger.Println("Stats certificate reloader could not connect to Vault: no modifier returned")
						if vault != nil {
							vault.Close()
							vault = nil
						}
						continue
					}
				}
				if err := reloadStatsServerCertificate(driverConfig, mod, port); err != nil {
					logger.Printf("Stats certificate reload check failed: %v", err)
					mod.Release()
					mod = nil
					if vault != nil {
						vault.Close()
						vault = nil
					}
				}
			}
		}()
	})
}

func reloadStatsServerCertificate(driverConfig *config.DriverConfig, mod *helperkv.Modifier, port int) error {
	certCache := driverConfig.CoreConfig.CertCache
	if certCache == nil {
		return errors.New("certificate cache is unavailable")
	}
	certPath := tccore.TRCSHHIVEK_CERT
	keyPath := tccore.TRCSHHIVEK_KEY
	oldCert, certCached := certCache.Get(certPath)
	oldKey, keyCached := certCache.Get(keyPath)
	if !certCached || oldCert == nil || oldCert.CertBytes == nil || !keyCached || oldKey == nil || oldKey.CertBytes == nil {
		return errors.New("stats server certificate pair is missing from cache")
	}
	readCreatedTime := func(path string) (any, error) {
		basis := strings.TrimPrefix(path, "Common/")
		basis = strings.TrimSuffix(basis, ".crt.mf.tmpl")
		basis = strings.TrimSuffix(basis, ".key.mf.tmpl")
		metadata, err := mod.ReadMetadata(fmt.Sprintf("values/%s", basis), driverConfig.CoreConfig.Log)
		if err != nil {
			return nil, err
		}
		createdTime, ok := metadata["created_time"]
		if !ok {
			return nil, fmt.Errorf("Vault metadata has no created_time for %s", path)
		}
		return createdTime, nil
	}
	certCreatedTime, err := readCreatedTime(certPath)
	if err != nil {
		return err
	}
	keyCreatedTime, err := readCreatedTime(keyPath)
	if err != nil {
		return err
	}
	if fmt.Sprint(certCreatedTime) == fmt.Sprint(oldCert.CreatedTime) && fmt.Sprint(keyCreatedTime) == fmt.Sprint(oldKey.CreatedTime) {
		return nil
	}

	logger.Println("Updated stats server certificate metadata detected; loading the new certificate pair")
	certCache.Evict(certPath)
	certCache.Evict(keyPath)
	_, err = certutil.AddToCache(certPath, driverConfig, mod)
	if err == nil {
		_, err = certutil.AddToCache(keyPath, driverConfig, mod)
	}
	if err != nil {
		certCache.Set(certPath, oldCert)
		certCache.Set(keyPath, oldKey)
		return fmt.Errorf("unable to load updated stats certificate pair: %w", err)
	}
	newCert, certOK := certCache.Get(certPath)
	newKey, keyOK := certCache.Get(keyPath)
	if !certOK || newCert == nil || newCert.CertBytes == nil || !keyOK || newKey == nil || newKey.CertBytes == nil {
		certCache.Set(certPath, oldCert)
		certCache.Set(keyPath, oldKey)
		return errors.New("updated stats certificate pair was not fully cached")
	}
	if _, err := tls.X509KeyPair(*newCert.CertBytes, *newKey.CertBytes); err != nil {
		certCache.Set(certPath, oldCert)
		certCache.Set(keyPath, oldKey)
		return fmt.Errorf("updated stats certificate and key do not match: %w", err)
	}
	if bytes.Equal(*oldCert.CertBytes, *newCert.CertBytes) && bytes.Equal(*oldKey.CertBytes, *newKey.CertBytes) {
		logger.Println("Stats certificate metadata changed but certificate bytes are unchanged; no restart needed")
		return nil
	}
	if err := restartStatsServer(port, *newCert.CertBytes, *newKey.CertBytes, *oldCert.CertBytes, *oldKey.CertBytes); err != nil {
		certCache.Set(certPath, oldCert)
		certCache.Set(keyPath, oldKey)
		return err
	}
	return nil
}

func StatServerInit(trcshDriverConfig *capauth.TrcshDriverConfig, pluginConfig map[string]any) error {
	var goMod *helperkv.Modifier
	var vault *sys.Vault
	var err error

	// Grabbing configs
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
	reloadVaultAddress := pluginConfig["vaddress"]
	reloadTokenPtr := pluginConfig["tokenptr"]
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
		fmt.Fprintf(os.Stderr, "Server listening on :%d\n", trcstatsport)
		statCert, err := certutil.AddToCache(tccore.TRCSHHIVEK_CERT,
			trcshDriverConfig.DriverConfig,
			goMod)
		if err != nil || statCert == nil {
			if err == nil {
				err = errors.New("statCert is nil")
			}
			log.Printf("Couldn't load cert: %v", err)
			return err
		}
		statKey, err := certutil.AddToCache(tccore.TRCSHHIVEK_KEY,
			trcshDriverConfig.DriverConfig,
			goMod)
		if err != nil || statKey == nil {
			if err == nil {
				err = errors.New("statKey is nil")
			}
			log.Printf("Couldn't load key: %v", err)
			return err
		}

		// Initialize the stats.
		InitStats()

		statsServerMu.Lock()
		err = startStatsServer(trcstatsport, *statCert, *statKey)
		statsServerMu.Unlock()
		if err != nil {
			logger.Printf("Failed to start stats gRPC service: %v", err)
			return err
		}
		reloadPluginConfig := map[string]any{
			"vaddress": reloadVaultAddress,
			"tokenptr": reloadTokenPtr,
			"env":      trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis,
		}
		startStatsCertReloader(trcshDriverConfig.DriverConfig, reloadPluginConfig, trcstatsport)
	}
	return nil
}
