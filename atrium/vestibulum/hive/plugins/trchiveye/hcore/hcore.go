package hcore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/evanw/esbuild/pkg/api"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/trimble-oss/tierceron-core/v2/buildopts/plugincoreopts"
	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	trcshfs "github.com/trimble-oss/tierceron-core/v2/trcshfs"
	"github.com/trimble-oss/tierceron-core/v2/trcshfs/trcshio"
	"gopkg.in/yaml.v2"
)

var (
	configContext *tccore.ConfigContext
	pluginNameVar string
	sender        chan error
	dfstat        *tccore.TTDINode
	httpServer    *http.Server

	jsAssetRegistry      = cmap.New[string]()
	jsAssetTemplatePath  = cmap.New[string]()
	templateCompileCache = cmap.New[*compiledJSAsset]()
	runtimeConfig        = cmap.New[any]()
	runtimeChecksums     = cmap.New[string]()
	compilerMu           sync.Mutex
	compileMemFS         trcshio.MemoryFileSystem = trcshfs.NewTrcshMemFs()
	compileMemFSMu       sync.Mutex
)

type compiledJSAsset struct {
	TemplatePath string
	AssetPath    string
	SourceSHA256 string
	JSCode       string
	CompiledAt   time.Time
}

const (
	COMMON_PATH      = "./config.yml"
	templateRootPath = "./trc_templates"
)

func receiverHiveye(receiveChan chan tccore.KernelCmd) {
	for {
		event := <-receiveChan
		switch {
		case event.Command == tccore.PLUGIN_EVENT_START:
			go configContext.Start(event.PluginName)
		case event.Command == tccore.PLUGIN_EVENT_STOP:
			go stop()
			sender <- errors.New("hiveye shutting down")
			return
		case event.Command == tccore.PLUGIN_EVENT_STATUS:
			// TODO
		default:
			// TODO
		}
	}
}

func chat_receiver(chatReceiveChan chan *tccore.ChatMsg) {
	for {
		event := <-chatReceiveChan
		switch {
		case event == nil:
			continue
		case event.Name != nil && *event.Name == "SHUTDOWN":
			if configContext != nil {
				configContext.Log.Println("hiveye shutting down message receiver")
			}
			return
		default:
			if configContext != nil {
				configContext.Log.Println("hiveye received chat message")
			}
		}
	}
}

func init() {
	if plugincoreopts.BuildOptions.IsPluginHardwired() {
		return
	}
	peerExe, err := os.Open("plugins/hiveye.so")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Hiveye unable to sha256 plugin")
		return
	}
	defer peerExe.Close()

	h := sha256.New()
	if _, err := io.Copy(h, peerExe); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to copy file for sha256 of plugin: %s\n", err)
		return
	}
	sha := hex.EncodeToString(h.Sum(nil))
	fmt.Fprintf(os.Stderr, "Hiveye Version: %s\n", sha)
}

func send_dfstat() {
	if configContext == nil || configContext.DfsChan == nil || dfstat == nil {
		fmt.Fprintln(os.Stderr, "Dataflow statistic channel not initialized properly for hiveye.")
		return
	}
	dfsctx, _, err := dfstat.GetDeliverStatCtx()
	if err != nil {
		configContext.Log.Println("Failed to get dataflow statistic context: ", err)
		send_err(err)
		return
	}
	tccore.SendDfStat(configContext, dfsctx, dfstat)
}

func send_err(err error) {
	if configContext == nil || configContext.ErrorChan == nil || err == nil {
		fmt.Fprintln(os.Stderr, "Failure to send error message, error channel not initialized properly for hiveye.")
		return
	}
	if dfstat != nil {
		dfsctx, _, statErr := dfstat.GetDeliverStatCtx()
		if statErr != nil {
			configContext.Log.Println("Failed to get dataflow statistic context: ", statErr)
		} else {
			dfstat.UpdateDataFlowStatistic(dfsctx.FlowGroup,
				dfsctx.FlowName,
				dfsctx.StateName,
				dfsctx.StateCode,
				2,
				func(msg string, err error) {
					configContext.Log.Println(msg, err)
				})
			tccore.SendDfStat(configContext, dfsctx, dfstat)
		}
	}
	*configContext.ErrorChan <- err
}

func clearJSAssets() {
	jsAssetRegistry.Clear()
	jsAssetTemplatePath.Clear()
	templateCompileCache.Clear()
	runtimeConfig.Clear()
	runtimeChecksums.Clear()
}

func extractDriverMemFS(properties *map[string]any) trcshio.MemoryFileSystem {
	if properties == nil {
		return nil
	}

	if raw, ok := (*properties)["memfs"]; ok {
		if memFs, ok := raw.(trcshio.MemoryFileSystem); ok {
			return memFs
		}
	}

	if raw, ok := (*properties)["driverConfig"]; ok && raw != nil {
		rv := reflect.ValueOf(raw)
		if rv.Kind() == reflect.Ptr {
			if rv.IsNil() {
				return nil
			}
			rv = rv.Elem()
		}
		if rv.Kind() == reflect.Struct {
			memFsField := rv.FieldByName("MemFs")
			if memFsField.IsValid() && memFsField.CanInterface() {
				if memFs, ok := memFsField.Interface().(trcshio.MemoryFileSystem); ok {
					return memFs
				}
			}
		}
	}

	return nil
}

func configureCompileMemFS(properties *map[string]any) {
	compileMemFSMu.Lock()
	defer compileMemFSMu.Unlock()

	if driverMemFS := extractDriverMemFS(properties); driverMemFS != nil {
		compileMemFS = driverMemFS
		return
	}

	if compileMemFS == nil {
		compileMemFS = trcshfs.NewTrcshMemFs()
	}
}

func getCompileMemFS() trcshio.MemoryFileSystem {
	compileMemFSMu.Lock()
	defer compileMemFSMu.Unlock()
	if compileMemFS == nil {
		compileMemFS = trcshfs.NewTrcshMemFs()
	}
	return compileMemFS
}

func copyStringAnyMap(source map[string]any) map[string]any {
	cloned := map[string]any{}
	for k, v := range source {
		cloned[k] = v
	}
	return cloned
}

func copyStringMap(source map[string]string) map[string]string {
	cloned := map[string]string{}
	for k, v := range source {
		cloned[k] = v
	}
	return cloned
}

func parseBoolConfig(raw any, defaultValue bool) bool {
	if raw == nil {
		return defaultValue
	}
	switch v := raw.(type) {
	case bool:
		return v
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		if err == nil {
			return parsed
		}
	}
	return defaultValue
}

func parseStringConfig(raw any, defaultValue string) string {
	if raw == nil {
		return defaultValue
	}
	if value, ok := raw.(string); ok {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return defaultValue
}

func templateToJSAssetPath(templatePath, prefix string) (string, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "/hiveye/js/"
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	assetPath := strings.TrimPrefix(templatePath, "trc_templates/")
	assetPath = filepath.ToSlash(assetPath)
	assetPath = strings.TrimPrefix(assetPath, "/")
	if assetPath == "" {
		return "", errors.New("empty asset path")
	}
	if strings.HasSuffix(assetPath, ".tmpl") {
		assetPath = strings.TrimSuffix(assetPath, ".tmpl")
	}
	if strings.HasSuffix(assetPath, ".ts") {
		assetPath = strings.TrimSuffix(assetPath, ".ts") + ".js"
	}

	return prefix + assetPath, nil
}

func registerJSAsset(templatePath, jsCode string, config map[string]any) (string, error) {
	prefix := parseStringConfig(config["hiveye_js_prefix"], "/hiveye/js/")
	assetPath, err := templateToJSAssetPath(templatePath, prefix)
	if err != nil {
		return "", err
	}

	jsAssetRegistry.Set(assetPath, jsCode)
	jsAssetTemplatePath.Set(assetPath, templatePath)
	return assetPath, nil
}

func registerTemplateLookup(templatePath string, config map[string]any) (string, error) {
	prefix := parseStringConfig(config["hiveye_js_prefix"], "/hiveye/js/")
	assetPath, err := templateToJSAssetPath(templatePath, prefix)
	if err != nil {
		return "", err
	}

	jsAssetTemplatePath.Set(assetPath, templatePath)
	return assetPath, nil
}

func getRuntimeCompileInputs() (map[string]any, map[string]string) {
	configSnapshot := map[string]any{}
	for item := range runtimeConfig.IterBuffered() {
		configSnapshot[item.Key] = item.Val
	}
	checksumSnapshot := map[string]string{}
	for item := range runtimeChecksums.IterBuffered() {
		checksumSnapshot[item.Key] = item.Val
	}
	return configSnapshot, checksumSnapshot
}

func serveJSAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jsCode, ok := jsAssetRegistry.Get(r.URL.Path)
	templatePath, _ := jsAssetTemplatePath.Get(r.URL.Path)
	if !ok {
		if templatePath == "" {
			http.NotFound(w, r)
			return
		}

		config, checksums := getRuntimeCompileInputs()
		entry, err := compileAndCacheTemplate(templatePath, config, checksums)
		if err != nil {
			if configContext != nil && configContext.Log != nil {
				configContext.Log.Printf("failed to compile javascript asset %s: %v", templatePath, err)
			}
			http.Error(w, "failed to compile javascript asset", http.StatusInternalServerError)
			return
		}
		jsCode = entry.JSCode
	}

	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, jsCode)
}

func startJSServerIfConfigured(config map[string]any) error {
	assetCount := jsAssetTemplatePath.Count()
	if assetCount == 0 {
		return nil
	}

	enabled := parseBoolConfig(config["hiveye_js_enabled"], parseBoolConfig(config["hiveye_http_enabled"], true))
	if !enabled {
		if configContext != nil && configContext.Log != nil {
			configContext.Log.Printf("hiveye JS server disabled by config; %d asset(s) prepared", assetCount)
		}
		return nil
	}

	listenAddr := parseStringConfig(config["hiveye_js_listen"], parseStringConfig(config["hiveye_http_listen"], ":8088"))
	mux := http.NewServeMux()
	mux.HandleFunc("/", serveJSAsset)
	httpServer = &http.Server{
		Addr:              listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	if configContext != nil && configContext.Log != nil {
		configContext.Log.Printf("hiveye JS server starting on %s with %d asset(s)", listenAddr, assetCount)
	}

	go func(server *http.Server) {
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			if configContext != nil && configContext.Log != nil {
				configContext.Log.Printf("hiveye JS server failed: %v", err)
			}
			send_err(err)
		}
	}(httpServer)

	return nil
}

func compileAndCacheTemplate(templatePath string, config map[string]any, checksums map[string]string) (*compiledJSAsset, error) {
	compilerMu.Lock()
	defer compilerMu.Unlock()

	normalizedPath, err := normalizeAndValidateTemplatePath(templatePath)
	if err != nil {
		return nil, err
	}
	templatePath = normalizedPath

	source, err := renderTypeScriptTemplate(templatePath, config)
	if err != nil {
		return nil, fmt.Errorf("rendering typescript template %s failed: %w", templatePath, err)
	}
	if err := validateTemplateChecksum(templatePath, source, checksums); err != nil {
		return nil, err
	}

	hash := sha256.Sum256([]byte(source))
	sourceSHA := hex.EncodeToString(hash[:])
	assetPath, err := templateToJSAssetPath(templatePath, parseStringConfig(config["hiveye_js_prefix"], "/hiveye/js/"))
	if err != nil {
		return nil, err
	}

	if cached, found := templateCompileCache.Get(templatePath); found && cached.SourceSHA256 == sourceSHA && cached.AssetPath == assetPath {
		return cached, nil
	}

	jsCode, err := transpileTypeScriptWithTsgo(templatePath, source)
	if err != nil {
		return nil, err
	}

	entry := &compiledJSAsset{
		TemplatePath: templatePath,
		AssetPath:    assetPath,
		SourceSHA256: sourceSHA,
		JSCode:       jsCode,
		CompiledAt:   time.Now().UTC(),
	}

	jsAssetRegistry.Set(assetPath, jsCode)
	jsAssetTemplatePath.Set(assetPath, templatePath)
	templateCompileCache.Set(templatePath, entry)

	if configContext != nil && configContext.Log != nil {
		configContext.Log.Printf("[hiveye-js] compiled %s -> %s", templatePath, assetPath)
	}

	return entry, nil
}

func discoverTypeScriptTemplates() ([]string, error) {
	fs := getCompileMemFS()
	templates := make([]string, 0)
	err := fs.Walk(templateRootPath, func(path string, isDir bool) error {
		path = filepath.ToSlash(path)
		if strings.HasPrefix(path, "./") {
			path = strings.TrimPrefix(path, "./")
		}
		if isDir {
			return nil
		}
		if strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".ts.tmpl") {
			templates = append(templates, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(templates) == 0 {
		err = fs.Walk(".", func(path string, isDir bool) error {
			path = filepath.ToSlash(path)
			if strings.HasPrefix(path, "./") {
				path = strings.TrimPrefix(path, "./")
			}
			if isDir {
				return nil
			}
			if strings.HasPrefix(path, "trc_templates/") && (strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".ts.tmpl")) {
				templates = append(templates, path)
			}
			return nil
		})
	}
	if err != nil {
		return nil, err
	}
	sort.Strings(templates)
	return templates, nil
}

func loadConfiguredTemplates(config map[string]any) ([]string, bool) {
	raw, ok := config["typescript_templates"]
	if !ok || raw == nil {
		return nil, false
	}
	configured := make([]string, 0)
	switch value := raw.(type) {
	case []string:
		configured = append(configured, value...)
	case []any:
		for _, item := range value {
			asString, castOK := item.(string)
			if castOK && asString != "" {
				configured = append(configured, asString)
			}
		}
	}
	return configured, len(configured) > 0
}

func normalizeAndValidateTemplatePath(templatePath string) (string, error) {
	if templatePath == "" {
		return "", errors.New("empty template path")
	}
	if filepath.IsAbs(templatePath) {
		return "", fmt.Errorf("absolute template path is not allowed: %s", templatePath)
	}

	cleaned := filepath.Clean(templatePath)
	cleanedSlash := filepath.ToSlash(cleaned)
	if strings.HasPrefix(cleanedSlash, "../") || cleanedSlash == ".." {
		return "", fmt.Errorf("template path escapes repository root: %s", templatePath)
	}
	if !strings.HasPrefix(cleanedSlash, "trc_templates/") {
		return "", fmt.Errorf("template path must be under trc_templates: %s", templatePath)
	}
	if !(strings.HasSuffix(cleanedSlash, ".ts") || strings.HasSuffix(cleanedSlash, ".ts.tmpl")) {
		return "", fmt.Errorf("template path must end with .ts or .ts.tmpl: %s", templatePath)
	}

	return cleanedSlash, nil
}

func loadTemplateChecksums(config map[string]any) map[string]string {
	checksums := map[string]string{}
	raw, ok := config["typescript_template_sha256"]
	if !ok || raw == nil {
		return checksums
	}

	if typedMap, ok := raw.(map[string]any); ok {
		for key, value := range typedMap {
			normalized, err := normalizeAndValidateTemplatePath(key)
			if err != nil {
				continue
			}
			if checksum, ok := value.(string); ok {
				checksum = strings.TrimSpace(strings.ToLower(checksum))
				if checksum != "" {
					checksums[normalized] = checksum
				}
			}
		}
	}

	return checksums
}

func validateTemplateChecksum(templatePath, source string, checksums map[string]string) error {
	expected, ok := checksums[templatePath]
	if !ok {
		return nil
	}
	hash := sha256.Sum256([]byte(source))
	actual := hex.EncodeToString(hash[:])
	if subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) != 1 {
		return fmt.Errorf("template checksum mismatch for %s", templatePath)
	}
	return nil
}

func renderTypeScriptTemplate(templatePath string, config map[string]any) (string, error) {
	fs := getCompileMemFS()
	rawBytes, err := readMemFSFile(fs, templatePath)
	if err != nil {
		return "", err
	}

	rawContent := string(rawBytes)
	if !strings.HasSuffix(templatePath, ".tmpl") {
		return rawContent, nil
	}

	tmpl, err := template.New(filepath.Base(templatePath)).Option("missingkey=default").Parse(rawContent)
	if err != nil {
		return "", err
	}

	ctx := map[string]any{
		"plugin": pluginNameVar,
		"config": config,
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, ctx); err != nil {
		return "", err
	}
	return out.String(), nil
}

func readMemFSFile(fs trcshio.MemoryFileSystem, path string) ([]byte, error) {
	file, err := fs.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

func writeMemFSFile(fs trcshio.MemoryFileSystem, path string, data []byte) error {
	file, err := fs.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return err
	}
	return nil
}

func transpileTypeScriptWithTsgo(templatePath, source string) (string, error) {
	hash := sha256.Sum256([]byte(source))
	hashText := hex.EncodeToString(hash[:])
	sourcePath := "ts_" + hashText + ".ts"
	outputPath := "js_" + hashText + ".js"
	fs := getCompileMemFS()

	if err := writeMemFSFile(fs, sourcePath, []byte(source)); err != nil {
		return "", fmt.Errorf("writing memfs typescript source for %s failed: %w", templatePath, err)
	}

	result := api.Transform(source, api.TransformOptions{
		Sourcefile: sourcePath,
		Loader:     api.LoaderTS,
		Target:     api.ES2020,
		Format:     api.FormatESModule,
		LogLevel:   api.LogLevelSilent,
	})
	if len(result.Errors) > 0 {
		return "", fmt.Errorf("typescript transpile failed for %s: %s", templatePath, result.Errors[0].Text)
	}

	if err := writeMemFSFile(fs, outputPath, result.Code); err != nil {
		return "", fmt.Errorf("writing memfs javascript output for %s failed: %w", templatePath, err)
	}
	jsCode, err := readMemFSFile(fs, outputPath)
	if err != nil {
		return "", fmt.Errorf("reading memfs javascript output for %s failed: %w", templatePath, err)
	}

	return string(jsCode), nil
}

func runTypeScriptTemplates(config map[string]any) error {
	clearJSAssets()

	templates, configured := loadConfiguredTemplates(config)
	if !configured {
		var err error
		templates, err = discoverTypeScriptTemplates()
		if err != nil {
			return err
		}
	}
	if len(templates) == 0 {
		if configContext != nil && configContext.Log != nil {
			configContext.Log.Println("No TypeScript templates found for hiveye.")
		}
		return nil
	}

	checksums := loadTemplateChecksums(config)
	runtimeConfig.Clear()
	for key, value := range config {
		runtimeConfig.Set(key, value)
	}
	runtimeChecksums.Clear()
	for key, value := range checksums {
		runtimeChecksums.Set(key, value)
	}

	for _, templatePath := range templates {
		normalizedPath, err := normalizeAndValidateTemplatePath(templatePath)
		if err != nil {
			return err
		}
		templatePath = normalizedPath

		if _, err := registerTemplateLookup(templatePath, config); err != nil {
			return err
		}

		if configContext != nil && configContext.Log != nil {
			configContext.Log.Printf("Compiling TypeScript template: %s", templatePath)
		}
		if _, err := compileAndCacheTemplate(templatePath, config, checksums); err != nil {
			return err
		}
	}

	if err := startJSServerIfConfigured(config); err != nil {
		return err
	}
	return nil
}

func start(pluginName string) {
	if configContext == nil {
		fmt.Fprintln(os.Stderr, "no config context initialized for hiveye")
		return
	}

	var config map[string]any
	if configMap, ok := (*configContext.Config)[COMMON_PATH].(*map[string]any); ok {
		config = *configMap
	} else if configMap, ok := (*configContext.Config)[COMMON_PATH].(map[string]any); ok {
		config = configMap
	} else {
		configBytes, ok := (*configContext.Config)[COMMON_PATH].([]byte)
		if !ok {
			err := errors.New("missing common configs")
			configContext.Log.Println(err)
			send_err(err)
			return
		}
		if err := yaml.Unmarshal(configBytes, &config); err != nil {
			configContext.Log.Println("Missing common configs")
			send_err(err)
			return
		}
	}

	go func(cmdSendChan *chan tccore.KernelCmd) {
		if cmdSendChan != nil {
			*cmdSendChan <- tccore.KernelCmd{PluginName: pluginName, Command: tccore.PLUGIN_EVENT_START}
		}
	}(configContext.CmdSenderChan)

	dfstat = tccore.InitDataFlow(nil, configContext.ArgosId, false)
	dfstat.UpdateDataFlowStatistic("System",
		pluginName,
		"TypeScript engine start",
		"1",
		1,
		func(msg string, err error) {
			configContext.Log.Println(msg, err)
		})
	send_dfstat()

	if err := runTypeScriptTemplates(config); err != nil {
		configContext.Log.Printf("TypeScript template execution failed: %v", err)
		send_err(err)
		return
	}

	dfstat.UpdateDataFlowStatistic("System",
		pluginName,
		"TypeScript engine complete",
		"0",
		1,
		func(msg string, err error) {
			configContext.Log.Println(msg, err)
		})
	send_dfstat()
}

func stop() {
	if configContext != nil {
		if httpServer != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := httpServer.Shutdown(shutdownCtx); err != nil && configContext.Log != nil {
				configContext.Log.Printf("hiveye JS server shutdown error: %v", err)
			}
			cancel()
			httpServer = nil
		}

		configContext.Log.Println("Hiveye received shutdown message from kernel.")
		dfstat.UpdateDataFlowStatistic("System",
			pluginNameVar,
			"Shutdown",
			"0",
			1,
			func(msg string, err error) {
				if err != nil {
					configContext.Log.Println(tccore.SanitizeForLogging(err.Error()))
				} else {
					configContext.Log.Println(tccore.SanitizeForLogging(msg))
				}
			})
		send_dfstat()
		*configContext.CmdSenderChan <- tccore.KernelCmd{PluginName: pluginNameVar, Command: tccore.PLUGIN_EVENT_STOP}
	}
	dfstat = nil
	clearJSAssets()
}

func GetConfigContext(pluginName string) *tccore.ConfigContext {
	return configContext
}

func GetConfigPaths(pluginName string) []string {
	return []string{
		COMMON_PATH,
		tccore.TRCSHHIVEK_CERT,
		tccore.TRCSHHIVEK_KEY,
	}
}

func PostInit(ctx *tccore.ConfigContext) {
	configContext = ctx
	configContext.Start = start
	sender = *configContext.ErrorChan
	go receiverHiveye(*configContext.CmdReceiverChan)
}

func Init(pluginName string, properties *map[string]any) {
	var err error
	pluginNameVar = pluginName
	configureCompileMemFS(properties)
	configContext, err = tccore.Init(properties,
		tccore.TRCSHHIVEK_CERT,
		tccore.TRCSHHIVEK_KEY,
		COMMON_PATH,
		"hiveplugin",
		start,
		receiverHiveye,
		chat_receiver,
	)
	if err != nil && properties != nil && (*properties)["log"] != nil {
		(*properties)["log"].(*log.Logger).Printf("Initialization error: %v", err)
		return
	}
	if _, ok := (*properties)[COMMON_PATH]; !ok {
		fmt.Fprintln(os.Stderr, "Missing common config components")
		return
	}
}

func GetPluginMessages(pluginName string) []string {
	return []string{}
}
