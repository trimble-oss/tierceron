package trctvbase

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/trimble-oss/tierceron-core/v2/buildopts/kernelopts"
	"github.com/trimble-oss/tierceron-core/v2/buildopts/memonly"
	"github.com/trimble-oss/tierceron-core/v2/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

var outWriter io.Writer = os.Stdout

func PrintVersion() {
	fmt.Fprintln(outWriter, "Version: "+"1.0")
}

// parseTvPath parses a vault path like "super-secrets/dev/DataFlowStatistics"
// into (modifierPath, env) where modifierPath is "super-secrets/DataFlowStatistics"
// and env is "dev". Paths with two segments like "super-secrets/dev" are treated
// as a mount plus env with an empty remainder.
func parseTvPath(userPath string) (string, string) {
	parts := strings.Split(userPath, "/")
	if len(parts) < 2 {
		return userPath, ""
	}
	remainder := ""
	if len(parts) > 2 {
		remainder = strings.Join(parts[2:], "/")
	}
	if remainder == "" {
		return parts[0], parts[1]
	}
	return parts[0] + "/" + remainder, parts[1]
}

func isHelpArg(arg string) bool {
	return arg == "-h" || arg == "--help" || arg == "help"
}

// setupOutputWriter redirects stdout to the MemFs STDIO file when running in shell mode.
// Returns a close function that should be deferred by the caller.
func setupOutputWriter(driverConfig *config.DriverConfig, flagset *flag.FlagSet) func() {
	if driverConfig == nil || !driverConfig.IsShellCommand || driverConfig.MemFs == nil {
		return func() {}
	}
	var stdioFile io.ReadWriteCloser
	var err error
	if _, statErr := driverConfig.MemFs.Stat("io"); statErr == nil {
		stdioFile, err = driverConfig.MemFs.OpenFile("io/STDIO", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o644)
	} else {
		emptyData := []byte{}
		driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &emptyData, "io/STDIO")
		stdioFile, err = driverConfig.MemFs.OpenFile("io/STDIO", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o644)
	}
	if err == nil {
		outWriter = stdioFile
		if flagset != nil {
			flagset.SetOutput(outWriter)
		}
		return func() {
			stdioFile.Close()
			outWriter = os.Stdout
		}
	}
	return func() {}
}

func CommonMain(
	envDefaultPtr *string,
	envCtxPtr *string,
	tokenNamePtr *string,
	regionPtr *string,
	flagset *flag.FlagSet,
	argLines []string,
	driverConfig *config.DriverConfig,
) error {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}

	isShellCmd := driverConfig != nil && driverConfig.IsShellCommand

	var tokenPtr *string
	var addrPtr *string
	var localEnvPtr *string

	if flagset == nil {
		if driverConfig == nil || driverConfig.CoreConfig == nil || !driverConfig.CoreConfig.IsEditor {
			PrintVersion()
		}
		errorHandling := flag.ExitOnError
		if driverConfig != nil && (isShellCmd || (kernelopts.BuildOptions != nil && kernelopts.BuildOptions.IsKernelZ())) {
			errorHandling = flag.ContinueOnError
		}
		flagset = flag.NewFlagSet(argLines[0], errorHandling)
		flagset.Usage = func() {
			fmt.Fprintf(flagset.Output(), "Usage of %s:\n", argLines[0])
			fmt.Fprintf(flagset.Output(), "  %s list <path>\n", argLines[0])
			fmt.Fprintf(flagset.Output(), "      List accessible entries at a secret store path without reading secret data\n")
			fmt.Fprintf(flagset.Output(), "      Path: <mount>/<env>/<path>  e.g. super-secrets/dev\n")
			fmt.Fprintf(flagset.Output(), "  %s get <path>\n", argLines[0])
			fmt.Fprintf(flagset.Output(), "      Read key/value data from a secret store path\n")
			fmt.Fprintf(flagset.Output(), "      Path: <mount>/<env>/<secret>  e.g. super-secrets/dev/DataFlowStatistics\n")
			fmt.Fprintf(flagset.Output(), "  %s patch <path> <key>=<value> ...\n", argLines[0])
			fmt.Fprintf(flagset.Output(), "      Update one or more keys at a secret store path (requires elevated access)\n")
			flagset.PrintDefaults()
		}
		localEnvPtr = flagset.String("env", "", "Environment override (default: embedded in path)")
	} else {
		tokenPtr = flagset.String("token", "", "Vault access token")
		addrPtr = flagset.String("addr", "", "API endpoint for the vault")
		localEnvPtr = envDefaultPtr // already registered by the caller (cmd/trctv main)
	}

	insecurePtr := flagset.Bool("insecure", false, "Allow insecure SSL connections")
	logFilePtr := flagset.String("log", "./"+coreopts.BuildOptions.GetFolderPrefix(nil)+"tv.log", "Output path for log file")
	pingPtr := flagset.Bool("ping", false, "Ping vault.")

	defer setupOutputWriter(driverConfig, flagset)()

	// --- Argument parsing ---
	var positionalArgs []string
	showHelp := false

	if isShellCmd {
		// Shell mode: manually extract recognised flags; treat others as positional.
		for i := 1; i < len(argLines); i++ {
			arg := argLines[i]
			switch {
			case isHelpArg(arg):
				showHelp = true
			case strings.HasPrefix(arg, "-env="):
				*localEnvPtr = arg[5:]
			case arg == "-env" && i+1 < len(argLines):
				i++
				*localEnvPtr = argLines[i]
			case arg == "-insecure" || arg == "--insecure":
				*insecurePtr = true
			case arg == "-ping" || arg == "--ping":
				*pingPtr = true
			case !strings.HasPrefix(arg, "-"):
				positionalArgs = append(positionalArgs, arg)
			}
		}
	} else {
		parseErr := flagset.Parse(argLines[1:])
		if parseErr == flag.ErrHelp {
			return nil
		}
		if parseErr != nil {
			return parseErr
		}
		positionalArgs = flagset.Args()
	}

	if showHelp {
		flagset.Usage()
		return nil
	}

	// sub-command, path
	if len(positionalArgs) < 2 {
		flagset.Usage()
		return errors.New("usage: tv <list|get|patch> <path> [key=value ...]")
	}
	subCmd := positionalArgs[0]
	userPath := positionalArgs[1]

	// --- Environment resolution ---
	vaultPath, pathEnv := parseTvPath(userPath)
	env := pathEnv
	if localEnvPtr != nil && *localEnvPtr != "" {
		// Explicit -env flag overrides embedded env.
		env = *localEnvPtr
		// When -env overrides, the userPath is used as-is (no env segment to strip).
		if pathEnv == "" {
			// path had no embedded env; keep vaultPath as-is
			vaultPath = userPath
		}
	}
	if env == "" && envDefaultPtr != nil && *envDefaultPtr != "" {
		env = *envDefaultPtr
	}
	if env == "" {
		env = driverConfig.CoreConfig.EnvBasis
	}
	if env == "" {
		env = "dev"
	}
	envBasis := eUtils.GetEnvBasis(env)

	// --- Logger setup ---
	var logger *log.Logger
	if driverConfig != nil && driverConfig.CoreConfig != nil && driverConfig.CoreConfig.Log != nil {
		logger = driverConfig.CoreConfig.Log
	} else {
		f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("error opening log file: %w", err)
		}
		defer f.Close()
		logger = log.New(f, "["+coreopts.BuildOptions.GetFolderPrefix(nil)+"tv]", log.LstdFlags)
	}

	// --- Token / auth setup ---
	tokenName := fmt.Sprintf("config_token_%s", envBasis)
	if tokenNamePtr != nil && *tokenNamePtr != "" {
		tokenName = *tokenNamePtr
	}

	if !isShellCmd {
		// Standalone binary: authenticate if no token provided directly.
		if driverConfig == nil {
			return errors.New("driverConfig must be provided")
		}
		driverConfig.CoreConfig.Insecure = *insecurePtr
		driverConfig.CoreConfig.Env = env
		driverConfig.CoreConfig.Log = logger
		if addrPtr != nil && *addrPtr != "" {
			driverConfig.CoreConfig.TokenCache.VaultAddressPtr = addrPtr
		}
		if tokenPtr != nil && *tokenPtr != "" {
			driverConfig.CoreConfig.TokenCache.AddToken(tokenName, tokenPtr)
		} else {
			roleEntity := "bamboo"
			autoErr := eUtils.AutoAuth(driverConfig, &tokenName, &tokenPtr, &env, envCtxPtr, &roleEntity, *pingPtr)
			if autoErr != nil {
				return fmt.Errorf("vault auth error: %w", autoErr)
			}
			if *pingPtr {
				fmt.Fprintln(outWriter, "Vault is reachable.")
				return nil
			}
		}
	}

	// --- Create Vault modifier ---
	mod, modErr := helperkv.NewModifierFromCoreConfig(driverConfig.CoreConfig, tokenName, envBasis, false)
	if modErr != nil {
		return fmt.Errorf("failed to create vault client: %w", modErr)
	}
	defer mod.Release()

	// Set Env so that path construction inside ReadData/Write uses the right prefix.
	mod.Env = envBasis

	// --- Dispatch sub-command ---
	switch subCmd {
	case "list":
		return executeList(vaultPath, mod, logger)
	case "get":
		return executeGet(vaultPath, mod, logger)
	case "patch":
		if len(positionalArgs) < 3 {
			fmt.Fprintln(outWriter, "Usage: tv patch <path> <key>=<value> [<key>=<value> ...]")
			return errors.New("patch requires at least one key=value argument")
		}
		return executePatch(vaultPath, positionalArgs[2:], mod, logger)
	default:
		flagset.Usage()
		return fmt.Errorf("unknown sub-command %q (expected 'list', 'get' or 'patch')", subCmd)
	}
}

// executeList lists accessible keys at vaultPath without reading secret values.
func executeList(vaultPath string, mod *helperkv.Modifier, logger *log.Logger) error {
	secret, err := mod.List(vaultPath, logger)
	if err != nil {
		fmt.Fprintf(outWriter, "Error listing %s: %v\n", vaultPath, err)
		return err
	}
	if secret == nil || secret.Data == nil {
		fmt.Fprintf(outWriter, "No entries found at: %s\n", vaultPath)
		return nil
	}

	rawKeys, ok := secret.Data["keys"]
	if !ok {
		fmt.Fprintf(outWriter, "No entries found at: %s\n", vaultPath)
		return nil
	}

	entries := []string{}
	switch keys := rawKeys.(type) {
	case []any:
		for _, key := range keys {
			entries = append(entries, fmt.Sprint(key))
		}
	case []string:
		entries = append(entries, keys...)
	default:
		fmt.Fprintf(outWriter, "Unexpected list response at %s\n", vaultPath)
		return nil
	}

	sort.Strings(entries)
	fmt.Fprintf(outWriter, "Path: %s\n", vaultPath)
	fmt.Fprintln(outWriter, "Entries:")
	for _, entry := range entries {
		fmt.Fprintf(outWriter, "  %s\n", entry)
	}
	return nil
}

// executeGet reads and prints the key/value data and metadata at vaultPath.
func executeGet(vaultPath string, mod *helperkv.Modifier, logger *log.Logger) error {
	data, err := mod.ReadData(vaultPath)
	if err != nil {
		fmt.Fprintf(outWriter, "Error reading %s: %v\n", vaultPath, err)
		return err
	}
	if data == nil {
		fmt.Fprintf(outWriter, "No data found at: %s\n", vaultPath)
		return nil
	}

	// Print sorted key=value pairs.
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Print metadata (version, timestamps, etc.) from the same KV v2 response.
	metadata, metaErr := mod.ReadMetadata(vaultPath, logger)
	if metaErr == nil && metadata != nil {
		fmt.Fprintln(outWriter, "Metadata:")
		metaKeys := make([]string, 0, len(metadata))
		for k := range metadata {
			metaKeys = append(metaKeys, k)
		}
		sort.Strings(metaKeys)
		for _, k := range metaKeys {
			fmt.Fprintf(outWriter, "%s = %v\n", k, metadata[k])
		}
		fmt.Fprintln(outWriter)
	}

	fmt.Fprintln(outWriter, "Data:")
	for _, k := range keys {
		fmt.Fprintf(outWriter, "%s = %v\n", k, data[k])
	}

	return nil
}

// executePatch reads the current data at vaultPath, applies the key=value updates, and writes back.
func executePatch(vaultPath string, kvPairs []string, mod *helperkv.Modifier, logger *log.Logger) error {
	// Read existing data so we perform a true patch (not a full replace).
	existing, err := mod.ReadData(vaultPath)
	if err != nil {
		fmt.Fprintf(outWriter, "Error reading %s before patch: %v\n", vaultPath, err)
		return err
	}
	if existing == nil {
		existing = make(map[string]any)
	}

	// Apply each key=value pair.
	updated := false
	for _, kv := range kvPairs {
		eqIdx := strings.Index(kv, "=")
		if eqIdx < 0 {
			fmt.Fprintf(outWriter, "Warning: skipping invalid argument (expected key=value): %s\n", kv)
			continue
		}
		key := kv[:eqIdx]
		value := kv[eqIdx+1:]
		existing[key] = value
		fmt.Fprintf(outWriter, "  %s = %s\n", key, value)
		updated = true
	}

	if !updated {
		fmt.Fprintln(outWriter, "No valid key=value pairs provided; nothing written.")
		return nil
	}

	// Write the full updated map back (Vault KV v2 replaces all fields on write).
	warnings, writeErr := mod.Write(vaultPath, existing, logger)
	if writeErr != nil {
		fmt.Fprintf(outWriter, "Error writing to %s: %v\n", vaultPath, writeErr)
		return writeErr
	}
	for _, w := range warnings {
		fmt.Fprintf(outWriter, "Warning: %s\n", w)
	}

	fmt.Fprintln(outWriter, "Patch successful.")
	return nil
}
