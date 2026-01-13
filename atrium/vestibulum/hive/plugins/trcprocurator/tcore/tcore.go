package tcore

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"time"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
)

var (
	configContext *tccore.ConfigContext
	proxyServer   *http.Server
	dfstat        *tccore.TTDINode
)

const (
	COMMON_PATH = "config"
)

func init() {
	peerExe, err := os.Open("/usr/local/trcshk/plugins/procurator.so")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Procurator unable to sha256 plugin")
		return
	}

	defer peerExe.Close()

	h := sha256.New()
	if _, err := io.Copy(h, peerExe); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to copy file for sha256 of plugin: %s\n", err)
		return
	}
	sha := hex.EncodeToString(h.Sum(nil))
	fmt.Fprintf(os.Stderr, "Procurator Version: %s\n", sha)
}

func send_dfstat() {
	if configContext == nil || configContext.DfsChan == nil || dfstat == nil {
		fmt.Fprintln(os.Stderr, "Dataflow Statistic channel not initialized properly for procurator.")
		return
	}
	dfsctx, _, err := dfstat.GetDeliverStatCtx()
	if err != nil {
		configContext.Log.Println("Failed to get dataflow statistic context: ", err)
		send_err(err)
		return
	}
	dfstat.Name = configContext.ArgosId
	dfstat.FinishStatistic("", "", "", configContext.Log, true, dfsctx)
	configContext.Log.Printf("Sending dataflow statistic to kernel: %s\n", dfstat.Name)
	dfstatClone := *dfstat
	go func(dsc *tccore.TTDINode) {
		if configContext != nil && *configContext.DfsChan != nil {
			*configContext.DfsChan <- dsc
		}
	}(&dfstatClone)
}

func send_err(err error) {
	if configContext == nil || configContext.ErrorChan == nil || err == nil {
		fmt.Fprintln(os.Stderr, "Failure to send error message, error channel not initialized properly for procurator.")
		return
	}
	if dfstat != nil {
		dfsctx, _, err := dfstat.GetDeliverStatCtx()
		if err != nil {
			configContext.Log.Println("Failed to get dataflow statistic context: ", err)
			return
		}
		dfstat.UpdateDataFlowStatistic(dfsctx.FlowGroup,
			dfsctx.FlowName,
			dfsctx.StateName,
			dfsctx.StateCode,
			2,
			func(msg string, err error) {
				configContext.Log.Println(msg, err)
			})
		dfstat.Name = configContext.ArgosId
		dfstat.FinishStatistic("", "", "", configContext.Log, true, dfsctx)
		configContext.Log.Printf("Sending failed dataflow statistic to kernel: %s\n", dfstat.Name)
		go func(sender chan *tccore.TTDINode, dfs *tccore.TTDINode) {
			sender <- dfs
		}(*configContext.DfsChan, dfstat)
	}
	*configContext.ErrorChan <- err
}

func chat_receiver(chat_receive_chan chan *tccore.ChatMsg) {
	for {
		event := <-chat_receive_chan
		switch {
		case event == nil:
			fallthrough
		case *event.Name == "SHUTDOWN":
			configContext.Log.Println("procurator shutting down message receiver")
			return
		case event.Response != nil && *event.Response == "Service unavailable":
			configContext.Log.Println("Procurator unable to access chat service.")
			return
		case event.ChatId != nil && (*event).ChatId != nil && *event.ChatId == "PROGRESS":
			configContext.Log.Println("procurator received progress chat message")
			response := "Running Procurator Proxy..."
			(*event).Response = &response
			*configContext.ChatSenderChan <- event
		default:
			configContext.Log.Println("procurator received chat message")
			response := ProcuratorDiagnostic()
			(*event).Response = &response
			*configContext.ChatSenderChan <- event
		}
	}
}

func ProcuratorDiagnostic() string {
	if configContext == nil {
		return "Improper config context for procurator diagnostic."
	}
	if proxyServer == nil {
		return "Procurator server not running."
	}
	return "Procurator proxy is running and forwarding HTTPS traffic to localhost."
}

func receiver(receive_chan chan tccore.KernelCmd) {
	for {
		event := <-receive_chan
		switch {
		case event.Command == tccore.PLUGIN_EVENT_START:
			go start(event.PluginName)
		case event.Command == tccore.PLUGIN_EVENT_STOP:
			go stop(event.PluginName)
			return
		case event.Command == tccore.PLUGIN_EVENT_STATUS:
		// TODO
		default:
			// TODO
		}
	}
}

// localhostOnlyHandler wraps a handler to only accept connections from localhost
type localhostOnlyHandler struct {
	handler http.Handler
	logger  *log.Logger
}

func (h *localhostOnlyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if len(r.Header) > 100 { // Max 100 headers
		h.logger.Printf("Blocked request with excessive headers from: %s", r.RemoteAddr)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		h.logger.Printf("Failed to parse RemoteAddr: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if r.TLS == nil {
		h.logger.Printf("Blocked non-TLS request from: %s", host)
		http.Error(w, "HTTPS Required", http.StatusUpgradeRequired)
		return
	}

	ip := net.ParseIP(host)
	if ip == nil || !ip.IsPrivate() {
		h.logger.Printf("Blocked non-private IP connection from: %s", host)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	h.handler.ServeHTTP(w, r)
}

func start(pluginName string) {
	if configContext == nil {
		fmt.Fprintln(os.Stderr, "no config context initialized for procurator")
		return
	}

	// Get configuration values
	var listenPort, targetPort int
	var err error

	if portInterface, ok := (*configContext.Config)["listen_port"]; ok {
		if port, ok := portInterface.(int); ok {
			listenPort = port
		} else {
			listenPort, err = strconv.Atoi(portInterface.(string))
			if err != nil {
				configContext.Log.Printf("Failed to process listen port: %v", err)
				send_err(err)
				return
			}
		}
	} else {
		configContext.Log.Println("Missing config: listen_port")
		send_err(errors.New("missing config: listen_port"))
		return
	}

	if portInterface, ok := (*configContext.Config)["target_port"]; ok {
		if port, ok := portInterface.(int); ok {
			targetPort = port
		} else {
			targetPort, err = strconv.Atoi(portInterface.(string))
			if err != nil {
				configContext.Log.Printf("Failed to process target port: %v", err)
				send_err(err)
				return
			}
		}
	} else {
		configContext.Log.Println("Missing config: target_port")
		send_err(errors.New("missing config: target_port"))
		return
	}

	// Validate ports
	if listenPort == targetPort {
		err := errors.New("listen_port and target_port must be different")
		configContext.Log.Println(err.Error())
		send_err(err)
		return
	}

	// Exclude well-known ports (0-1023) and ephemeral port ranges (49152+).
	// Safe range: 1024-49151 (registered ports, includes common app ports like 8000s)
	if listenPort < 1024 || listenPort > 49151 || targetPort < 1024 || targetPort > 49151 {
		err := errors.New("ports must be between 1024 and 49151 (excludes system and ephemeral ports)")
		configContext.Log.Println(err.Error())
		send_err(err)
		return
	}

	configContext.Log.Printf("Starting Procurator proxy: HTTPS :%d -> HTTPS 127.0.0.1:%d\n", listenPort, targetPort)

	// Create TLS configuration
	cert, err := tls.X509KeyPair(
		(*configContext.ConfigCerts)[tccore.TRCSHHIVEK_CERT],
		(*configContext.ConfigCerts)[tccore.TRCSHHIVEK_KEY])
	if err != nil {
		configContext.Log.Printf("Failed to load TLS certificate: %v", err)
		send_err(err)
		return
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		},
	}

	// Create target URL for reverse proxy (HTTPS only)
	targetURL, err := url.Parse(fmt.Sprintf("https://127.0.0.1:%d", targetPort))
	if err != nil {
		configContext.Log.Printf("Failed to parse target URL: %v", err)
		send_err(err)
		return
	}

	// Create reverse proxy with HTTPS transport
	// Always skip certificate verification for 127.0.0.1 (no valid cert will match localhost IP)
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.ErrorLog = configContext.Log
	proxy.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Required for 127.0.0.1 backend
		},
	}

	// Wrap with localhost-only middleware
	handler := &localhostOnlyHandler{
		handler: proxy,
		logger:  configContext.Log,
	}

	// Create HTTPS server
	proxyServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", listenPort),
		Handler:           handler,
		TLSConfig:         tlsConfig,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       180 * time.Second,
		ErrorLog:          configContext.Log,
		MaxHeaderBytes:    1 << 20,          // 1 MB max headers - header attack guard
		ReadHeaderTimeout: 10 * time.Second, // slow loris guard
	}

	// Start server in background
	go func() {
		configContext.Log.Printf("Procurator proxy listening on HTTPS port %d\n", listenPort)
		if err := proxyServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			configContext.Log.Printf("Proxy server error: %v", err)
			send_err(err)
		}
	}()

	dfstat = tccore.InitDataFlow(nil, configContext.ArgosId, false)
	dfstat.UpdateDataFlowStatistic("System",
		"procurator",
		"Start up",
		"1",
		1,
		func(msg string, err error) {
			configContext.Log.Println(msg, err)
		})
	send_dfstat()
}

func stop(pluginName string) {
	if configContext != nil {
		configContext.Log.Println("Procurator received shutdown message from kernel.")
		configContext.Log.Println("Stopping Procurator server")
	}
	if proxyServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := proxyServer.Shutdown(ctx); err != nil {
			configContext.Log.Printf("Error shutting down server: %v", err)
		}
	} else {
		fmt.Fprintln(os.Stderr, "no Procurator server initialized")
	}
	if configContext != nil {
		configContext.Log.Println("Stopped Procurator server")
		dfstat.UpdateDataFlowStatistic("System",
			"procurator",
			"Shutdown",
			"0",
			1, func(msg string, err error) {
				if err != nil {
					configContext.Log.Println(tccore.SanitizeForLogging(err.Error()))
				} else {
					configContext.Log.Println(tccore.SanitizeForLogging(msg))
				}
			})
		send_dfstat()
		*configContext.CmdSenderChan <- tccore.KernelCmd{PluginName: pluginName, Command: tccore.PLUGIN_EVENT_STOP}
	}
	proxyServer = nil
	dfstat = nil
}

func GetConfigContext(pluginName string) *tccore.ConfigContext { return configContext }

func Init(pluginName string, properties *map[string]any) {
	var err error
	configContext, err = tccore.Init(properties,
		tccore.TRCSHHIVEK_CERT,
		tccore.TRCSHHIVEK_KEY,
		COMMON_PATH,
		"hiveplugin",
		start,
		receiver,
		chat_receiver,
	)
	if err != nil {
		if configContext != nil {
			configContext.Log.Println("Successfully initialized procurator.")
			fmt.Fprintln(os.Stderr, err.Error())
		} else {
			fmt.Fprintln(os.Stderr, err.Error())
			return
		}
	}
	// Change logging context
	configContext.Log = log.New(configContext.Log.Writer(), "[procurator]", log.LstdFlags)
	configContext.Log.Println("Successfully initialized procurator.")
}

func GetConfigPaths(pluginName string) []string {
	return []string{
		COMMON_PATH,
		tccore.TRCSHHIVEK_CERT,
		tccore.TRCSHHIVEK_KEY,
	}
}

func GetPluginMessages(pluginName string) []string {
	return []string{}
}
