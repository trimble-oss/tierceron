package kv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/trcvault/opts/memonly"
	"github.com/trimble-oss/tierceron/utils/mlock"

	"github.com/hashicorp/vault/api"
)

// Set all paths that don't use environments to true
var noEnvironments = map[string]bool{
	"templates/": true,
	"cubbyhole/": true,
}

// Modifier maintains references to the active client and
// respective logical needed to write to the vault. Path
// can be changed to alter where in the vault the key,value
// pair is stored
type Modifier struct {
	Insecure         bool         // Indicates if connections to vault should be secure
	Direct           bool         // Bypass vault and utilize alternative source when possible.
	httpClient       *http.Client // Handle to http client.
	client           *api.Client  // Client connected to vault
	logical          *api.Logical // Logical used for read/write options
	Env              string       // Environment (local/dev/QA; Initialized to secrets)
	Regions          []string     // Supported regions
	SecretDictionary *api.Secret  // Current Secret Dictionary Cache.
	Version          string       // Version for data
	VersionFilter    []string     // Used to filter vault paths
	RawEnv           string
	TemplatePath     string   // Path to template we are processing.
	ProjectIndex     []string // Which projects are indexed.
	SectionKey       string   // The section key: Index or Restricted.
	SectionName      string   // The name of the actual section.
	SubSectionName   string   // The name of the actual subsection.
	SubSectionValue  string   // The actual value for the sub section.
	SectionPath      string   // The path to the Index (both seed and vault)
}

type modCache struct {
	modCount     uint64
	modifierChan chan *Modifier
}

var modifierCache map[string]*modCache = map[string]*modCache{}
var modifierCachLock sync.Mutex

// PreCheckEnvironment
// Returns: env, parts, true if parts is path, false if part of file name, error
func PreCheckEnvironment(environment string) (string, string, bool, error) {
	envParts := strings.Split(environment, ".")
	if len(envParts) == 2 {
		if envParts[1] != "*" {
			_, idErr := strconv.Atoi(envParts[1])
			if idErr != nil && len(envParts[1]) == 3 {
				return envParts[0], envParts[1], true, nil
			} else if idErr != nil {
				return "", "", false, idErr
			}
		}
		return envParts[0], envParts[1], false, nil
	} else if len(envParts) == 3 {
		return envParts[0], envParts[1], true, nil
	}

	return environment, "", false, nil
}

// NewModifier Constructs a new modifier struct and connects to the vault
// @param token 	The access token needed to connect to the vault
// @param address	The address of the API endpoint for the server
// @param env   	The environment currently connecting to.
// @param regions   Regions we want
// @param useCache Whether to use the modcache or not.
// @return 			A pointer to the newly contstructed modifier object (Note: path set to default),
//
//	Any errors generated in creating the client
func NewModifier(insecure bool, token string, address string, env string, regions []string, useCache bool, logger *log.Logger) (*Modifier, error) {
	if useCache {
		if _, ok := modifierCache[env]; !ok {
			modifierCachLock.Lock()
			modifierCache[env] = &modCache{modCount: 0, modifierChan: make(chan *Modifier, 20)}
			modifierCachLock.Unlock()
		}

		for {
			select {
			case checkoutModifier := <-modifierCache[env].modifierChan:
				return checkoutModifier, nil
			case <-time.After(time.Millisecond * 200):
				if atomic.LoadUint64(&modifierCache[env].modCount) < 20 {
					goto modbuild
				}
			}
		}
	modbuild:
		atomic.AddUint64(&modifierCache[env].modCount, 1)
	}

	if len(address) == 0 {
		address = "http://127.0.0.1:8020" // Default address
	}
	httpClient, err := CreateHTTPClient(insecure, address, env, false)
	if err != nil {
		return nil, err
	}
	// Create client
	modClient, err := api.NewClient(&api.Config{
		Address:    address,
		HttpClient: httpClient,
	})
	if err != nil {
		logger.Println("vaultHost: "+modClient.Address(), logger)
		return nil, err
	}

	// Set access token and path for this modifier
	modClient.SetToken(token)

	// Return the modifier
	newModifier := &Modifier{httpClient: httpClient, client: modClient, logical: modClient.Logical(), Env: "secret", RawEnv: env, Regions: regions, Version: "", Insecure: insecure}
	return newModifier, nil
}

func (m *Modifier) Release() {
	if _, ok := modifierCache[m.RawEnv]; !ok {
		modifierCachLock.Lock()
		modifierCache[m.RawEnv] = &modCache{modCount: 0, modifierChan: make(chan *Modifier, 20)}
		modifierCachLock.Unlock()
	}

	// TODO: Perform some maintenance???
	// Since modifiers are re-used now, this may not be necessary or even desired for that
	// matter.
	//	m.httpClient.CloseIdleConnections()

	modifierCache[m.RawEnv].modifierChan <- m
}

func (m *Modifier) RemoveFromCache() {
	m.Close()

	modifierCachLock.Lock()
	if modifierCache[m.RawEnv].modCount > 1 {
	emptied:
		for i := 0; i < 20; i++ {
			select {
			case <-modifierCache[m.RawEnv].modifierChan:
			default:
				break emptied
			}
		}
	}
	modifierCache[m.RawEnv].modCount = 0
	modifierCachLock.Unlock()
}

// ValidateEnvironment Ensures token has access to requested data.
func (m *Modifier) ValidateEnvironment(environment string, init bool, policySuffix string, logger *log.Logger) (bool, error) {
	env, sub, _, envErr := PreCheckEnvironment(environment)

	if envErr != nil {
		logger.Println(fmt.Sprintf("Environment format error: %v\n", envErr))
		return false, envErr
	} else {
		if sub != "" {
			environment = env
		}
	}

	if strings.Contains(environment, "local") {
		environment = "local"
	}
	desiredPolicy := "config_" + strings.ToLower(environment) + policySuffix

	if init {
		desiredPolicy = "vault_pub_" + strings.ToLower(environment)
	}

	secret, err := m.client.Auth().Token().LookupSelf()

	if err != nil {
		logger.Println(fmt.Sprintf("LookupSelf Auth failure: %v\n", err))
		if strings.Contains(err.Error(), "x509: certificate") {
			return false, err
		}
	}

	valid := false
	if err == nil {
		policies, _ := secret.TokenPolicies()

		for _, policy := range policies {
			if policy == "root" {
				valid = true
				break
			}
			if strings.ToLower(policy) == desiredPolicy {
				valid = true
				break
			}
		}

	}

	return valid, nil
}

// Writes the key,value pairs in data to the vault
//
// @param   data A set of key,value pairs to be written
//
// @return	Warnings (if any) generated from the vault,
//
//	errors generated by writing
func (m *Modifier) Write(path string, data map[string]interface{}, logger *log.Logger) ([]string, error) {
	// Wrap data and send
	sendData := map[string]interface{}{"data": data}

	// Create full path
	pathBlocks := strings.SplitAfterN(path, "/", 2)
	if len(pathBlocks) == 1 {
		pathBlocks[0] += "/"
	}
	fullPath := pathBlocks[0] + "data/"
	if !noEnvironments[pathBlocks[0]] { //if neither templates nor cubbyhole
		fullPath += m.Env + "/"

	} else if strings.HasPrefix(m.Env, "local") { //if local environment, add env to fullpath
		fullPath += m.Env + "/"
	}

	if m.SectionPath != "" && !strings.HasPrefix(fullPath, "templates") {
		fullPath += m.SectionPath + "/"
	}

	if len(pathBlocks) > 1 {
		if !strings.Contains(fullPath, "/"+pathBlocks[1]+"/") {
			fullPath += pathBlocks[1]
		}
	}

	if strings.Contains(fullPath, "/super-secrets/") {
		fullPath = strings.ReplaceAll(fullPath, "/super-secrets/", "/")
	}
	retries := 0
retryQuery:
	Secret, err := m.logical.Write(fullPath, sendData)
	if netErr, netErrOk := err.(*url.Error); netErrOk && netErr.Unwrap().Error() == "EOF" {
		if retries < 3 {
			retries = retries + 1
			goto retryQuery
		}
	} else if err == context.DeadlineExceeded || os.IsTimeout(err) {
		if retries < 3 {
			retries = retries + 1
			goto retryQuery
		}
	}
	if err != nil {
		logger.Printf("Modifier failing after %d retries.\n", retries)
	}

	if Secret == nil { // No warnings
		return nil, err
	}
	return Secret.Warnings, err
}

// ReadData Reads the most recent data from the path referenced by this Modifier
// @return	A Secret pointer that contains key,value pairs and metadata
//
//	errors generated from reading
func (m *Modifier) ReadData(path string) (map[string]interface{}, error) {
	bucket := path
	// Create full path
	if len(m.SectionPath) > 0 && !strings.HasPrefix(path, "templates") && !strings.HasPrefix(path, "value-metrics") { //Template paths are not indexed -> values & super-secrets are
		if strings.Contains(path, "values") {
			path = strings.Replace(m.SectionPath, "super-secrets", "values", -1)
		} else {
			path = m.SectionPath
		}
		path = strings.TrimSuffix(path, "/")
	}
	pathBlocks := strings.SplitAfterN(path, "/", 2)
	fullPath := pathBlocks[0] + "data/"
	if !noEnvironments[pathBlocks[0]] {
		fullPath += m.Env + "/"
	} else if strings.HasPrefix(m.Env, "local") { //if local environment, add env to retrieve correct path mod.Write wrote to
		fullPath += m.Env + "/"
	}
	if len(pathBlocks) > 1 {
		fullPath += pathBlocks[1]
	}
	retryCount := 0
retryVaultAccess:

	var secret *api.Secret
	var err error
	var versionMap = make(map[string][]string)
	if strings.HasSuffix(m.Version, "***X-Mode") { //x path
		if m.Version != "" && m.Version != "0" && strings.HasPrefix(path, "templates") {
			m.Version = strings.Split(m.Version, "***")[0]
			versionSlice := []string{m.Version}
			versionMap["version"] = versionSlice
			secret, err = m.logical.ReadWithData(fullPath, versionMap)
		}
	} else if m.Version != "" && !strings.HasPrefix(path, "templates") { //config path
		versionSlice := []string{m.Version}
		versionMap["version"] = versionSlice
		secret, err = m.logical.ReadWithData(fullPath, versionMap)
	} else {
		secret, err = m.logical.Read(fullPath)
	}

	if err != nil {
		if retryCount < 7 {
			retryCount = retryCount + 1
			goto retryVaultAccess
		}
	}

	if secret == nil {
		return nil, err
	}
	if data, ok := secret.Data["data"].(map[string]interface{}); ok {
		if memonly.IsMemonly() && !strings.HasPrefix(path, "templates") { // Don't lock templates
			for dataKey, dataValues := range data {
				if !buildopts.CheckMemLock(bucket, dataKey) {
					continue
				}
				if dataValuesSlice, isSlice := dataValues.([]interface{}); isSlice {
					for _, dataValues := range dataValuesSlice {
						if dataValueString, isString := dataValues.(string); isString {
							mlock.Mlock2(nil, &dataValueString)
						} else if _, isBool := dataValues.(bool); isBool {
							//mlock.Mlock2(nil, &dataValueString)
							// don't lock but accept bools.
						} else if _, isInt64 := dataValues.(int64); isInt64 {
							//mlock.Mlock2(nil, &dataValueString)
							// don't lock but accept int64.
						} else if _, isInt := dataValues.(int); isInt {
							//mlock.Mlock2(nil, &dataValueString)
							// don't lock but accept int.
						} else if _, isNumber := dataValues.(json.Number); isNumber {
							//mlock.Mlock2(nil, &dataValueString)
							// don't lock but accept json.Number.
						} else {
							return nil, errors.New(fmt.Sprintf("Unexpected datatype. Refusing to read what we cannot lock. Nested. %T", dataValues))
						}
					}
				} else if dataValueString, isString := dataValues.(string); isString {
					mlock.Mlock2(nil, &dataValueString)
				} else if _, isBool := dataValues.(bool); isBool {
					//mlock.Mlock2(nil, &dataValueString)
					// don't lock but accept bools.
				} else if _, isInt64 := dataValues.(int64); isInt64 {
					//mlock.Mlock2(nil, &dataValueString)
					// don't lock but accept int64.
				} else if _, isInt := dataValues.(int); isInt {
					//mlock.Mlock2(nil, &dataValueString)
					// don't lock but accept int.
				} else if _, isNumber := dataValues.(json.Number); isNumber {
					//mlock.Mlock2(nil, &dataValueString)
					// don't lock but accept json.Number.
				} else {
					return nil, errors.New(fmt.Sprintf("Unexpected datatype. Refusing to read what we cannot lock. %T", dataValues))
				}
			}
		}
		return data, err
	}
	return nil, errors.New("Could not get data from vault response")
}

// ReadMapValue takes a valueMap, path, and a key and returns the corresponding value from the vault
func (m *Modifier) ReadMapValue(valueMap map[string]interface{}, path string, key string) (string, error) {
	//return value corresponding to the key
	if valueMap[key] != nil {
		if value, ok := valueMap[key].(string); ok {
			return value, nil
		} else if stringer, ok := valueMap[key].(fmt.GoStringer); ok {
			return stringer.GoString(), nil
		} else {
			return "", fmt.Errorf("Cannot convert value at %s to string", key)
		}
	}
	return "", fmt.Errorf("Key '%s' not found in '%s'", key, path)
}

// ReadValue takes a path and a key and returns the corresponding value from the vault
func (m *Modifier) ReadValue(path string, key string) (string, error) {
	valueMap, err := m.ReadData(path)
	if err != nil {
		return "", err
	}
	return m.ReadMapValue(valueMap, path, key)
}

// ReadMetadata Reads the Metadata from the path referenced by this Modifier
// @return	A Secret pointer that contains key,value pairs and metadata
//
//	errors generated from reading
func (m *Modifier) ReadMetadata(path string, logger *log.Logger) (map[string]interface{}, error) {
	pathBlocks := strings.SplitAfterN(path, "/", 2)
	fullPath := pathBlocks[0] + "data/"
	if !noEnvironments[pathBlocks[0]] {
		fullPath += m.Env + "/"
	}
	fullPath += pathBlocks[1]
	retries := 0
retryQuery:
	secret, err := m.logical.Read(fullPath)
	if netErr, netErrOk := err.(*url.Error); netErrOk && netErr.Unwrap().Error() == "EOF" {
		if retries < 3 {
			retries = retries + 1
			goto retryQuery
		}
	} else if err == context.DeadlineExceeded || os.IsTimeout(err) {
		if retries < 3 {
			retries = retries + 1
			goto retryQuery
		}
	}
	if err != nil {
		logger.Printf("Modifier failing after %d retries.\n", retries)
	}

	if data, ok := secret.Data["metadata"].(map[string]interface{}); ok {
		return data, err
	}
	return nil, errors.New("Could not get metadata from vault response")
}

// ReadVersionMetadata Reads the Metadata of all versions from the path referenced by this Modifier
func (m *Modifier) ReadVersionMetadata(path string, logger *log.Logger) (map[string]interface{}, error) {
	// Create full path
	pathBlocks := strings.SplitAfterN(path, "/", 2)
	fullPath := pathBlocks[0] + "metadata/"

	if !noEnvironments[pathBlocks[0]] {
		fullPath += m.Env + "/"
	}
	if len(pathBlocks) > 1 {
		fullPath += pathBlocks[1]
	}
	retries := 0
retryQuery:
	secret, err := m.logical.Read(fullPath)
	if netErr, netErrOk := err.(*url.Error); netErrOk && netErr.Unwrap().Error() == "EOF" {
		if retries < 3 {
			retries = retries + 1
			goto retryQuery
		}
	} else if err == context.DeadlineExceeded || os.IsTimeout(err) {
		if retries < 3 {
			retries = retries + 1
			goto retryQuery
		}
	} else if secret == nil || secret.Data["versions"] == nil {
		return nil, errors.New("No version data.")
	}

	if err != nil {
		logger.Printf("Modifier failing after %d retries.\n", retries)
	}

	if versionsData, ok := secret.Data["versions"].(map[string]interface{}); ok {
		return versionsData, err
	}
	return nil, errors.New("Could not get metadata of versions from vault response")
}

// List lists the paths underneath this one
func (m *Modifier) List(path string, logger *log.Logger) (*api.Secret, error) {
	pathBlocks := strings.SplitAfterN(path, "/", 2)
	if len(pathBlocks) == 1 {
		pathBlocks[0] += "/"
	}

	fullPath := pathBlocks[0] + "metadata"
	if !noEnvironments[pathBlocks[0]] {
		fullPath += "/" + m.Env + "/"
	} else if strings.HasPrefix(m.Env, "local") { //if local environment, add env to fullpath
		fullPath += "/" + m.Env + "/"
	}
	if len(pathBlocks) > 1 {
		if !strings.HasSuffix(fullPath, "/") {
			fullPath += "/"
		}
		fullPath += pathBlocks[1]
	}
	retries := 0
retryQuery:
	result, err := m.logical.List(fullPath)
	if netErr, netErrOk := err.(*url.Error); netErrOk && netErr.Unwrap().Error() == "EOF" {
		if retries < 3 {
			retries = retries + 1
			goto retryQuery
		}
	} else if err == context.DeadlineExceeded || os.IsTimeout(err) {
		if retries < 3 {
			retries = retries + 1
			goto retryQuery
		}
	}
	if err != nil {
		logger.Printf("Modifier failing after %d retries.\n", retries)
		logger.Printf(err.Error())
	}
	return result, err
}

// List lists the paths underneath this one
func (m *Modifier) ListEnv(path string, logger *log.Logger) (*api.Secret, error) {
	pathBlocks := strings.SplitAfterN(path, "/", 2)
	var fullPath string
	if len(pathBlocks) == 1 {
		pathBlocks[0] += "/"
		fullPath = pathBlocks[0] + "metadata/"
	} else if len(pathBlocks) == 2 {
		fullPath = pathBlocks[0] + "metadata/"
		fullPath = fullPath + pathBlocks[1]
	}
	retries := 0
retryQuery:
	result, err := m.logical.List(fullPath)
	if netErr, netErrOk := err.(*url.Error); netErrOk && netErr.Unwrap().Error() == "EOF" {
		if retries < 3 {
			retries = retries + 1
			goto retryQuery
		}
	} else if err == context.DeadlineExceeded || os.IsTimeout(err) {
		if retries < 3 {
			retries = retries + 1
			goto retryQuery
		}
	}
	if err != nil {
		logger.Printf("Modifier failing after %d retries.\n", retries)
	}

	return result, err
}

// AdjustValue adjusts the value at the given path/key by n
func (m *Modifier) AdjustValue(path string, data map[string]interface{}, n int, logger *log.Logger) ([]string, error) {
	// Get the existing data at the path
	oldData, err := m.ReadData(path)
	if err != nil {
		return nil, err
	}
	if oldData == nil { // Path has not been used yet, create an empty map
		oldData = make(map[string]interface{})
	}
	for _, v := range data {
		if templateKey, ok := v.([]interface{}); ok {
			metricsKey := templateKey[0].(string) + "." + templateKey[1].(string)
			// Try to fetch the value with the given key, start empty values with 0
			if oldData[metricsKey] == nil {
				oldData[metricsKey] = "0"
			}
			// Convert from stored string value to int
			oldValue, err := strconv.Atoi(oldData[metricsKey].(string))
			if err != nil {
				logger.Printf("Could not convert value to int at: " + metricsKey)
				continue
			}
			newValue := strconv.Itoa(oldValue + n)
			oldData[metricsKey] = newValue
		}
	}
	return m.Write(path, oldData, logger)
}

// Proper shutdown of modifier.
func (m *Modifier) Close() {
	m.httpClient.CloseIdleConnections()
}

func (m *Modifier) Exists(path string) bool {
	secret, err := m.logical.List(path)

	if err != nil {
		return false
	}

	if secret == nil {
		return false
	} else {
		return true
	}
}

// GetProjectServiceMap - returns a map of all projects with list of their available services.
func (m *Modifier) GetProjectServicesMap(logger *log.Logger) (map[string][]string, error) {
	projectServiceMap := map[string][]string{}
	projectData, err := m.List("templates", logger)
	if err != nil {
		return nil, err
	}

	availProjects := projectData.Data["keys"].([]interface{})
	for _, availProject := range availProjects {
		serviceData, serviceErr := m.List("templates/"+availProject.(string), logger)
		if err != nil {
			return nil, serviceErr
		}

		availServices := serviceData.Data["keys"].([]interface{})
		services := []string{}
		for _, availService := range availServices {
			services = append(services, strings.ReplaceAll(availService.(string), "/", ""))
		}
		projectServiceMap[strings.ReplaceAll(availProject.(string), "/", "")] = services
	}

	return projectServiceMap, nil
}

// GetVersionValues gets filepath for values and grabs metadata for those paths.
func (m *Modifier) GetVersionValues(mod *Modifier, wantCerts bool, enginePath string, logger *log.Logger) (map[string]map[string]interface{}, error) {
	envCheck := make([]string, 2)
	var realEnv string
	lastIndex := strings.LastIndex(mod.Env, "_")
	if lastIndex != -1 {
		envCheck[0] = mod.Env[:lastIndex]
		envCheck[1] = mod.Env[lastIndex+1:]
		mod.Env = envCheck[0]
	} else {
		realEnv = mod.Env
	}

	if len(mod.ProjectIndex) > 0 {
		enginePath = enginePath + "/Index/" + mod.ProjectIndex[0] + "/" + mod.SectionName + "/" + mod.SubSectionValue
		mod.Env = mod.RawEnv
	}
	userPaths, err := mod.List(enginePath+"/", logger)
	versionDataMap := make(map[string]map[string]interface{}, 0)
	//data := make([]string, 0)
	if err != nil {
		return nil, err
	}
	if userPaths == nil {
		return nil, err
	}

	if wantCerts {
		//get a list of projects under values
		certPaths, err := m.getPaths("values/Common/", logger)
		if err != nil {
			return nil, err
		}

		for i, service := range mod.VersionFilter { //Cleans filter for cert metadata search
			if strings.Contains(service, "Common") {
				mod.VersionFilter[i] = strings.Replace(service, "Common", "Common/", 1)
			}
		}

		var filteredCertPaths []string
		for _, certPath := range certPaths { //Filter paths for optimization
			if certPath != "" {
				foundService := false
				for _, service := range mod.VersionFilter {
					if strings.HasSuffix(certPath, service) && !foundService {
						foundService = true
					}
				}

				if !foundService {
					continue
				} else {
					filteredCertPaths = append(filteredCertPaths, certPath)
				}
			}
		}

		certPaths = filteredCertPaths
		for _, certPath := range certPaths {
			if _, ok := versionDataMap[certPath]; !ok {
				metadataValue, err := mod.ReadVersionMetadata(certPath, logger)
				if err != nil {
					err := fmt.Errorf("Unable to fetch data from %s", certPath)
					return nil, err
				}
				if len(metadataValue) != 0 {
					versionDataMap[certPath] = metadataValue
				}
			}
		}
	} else {
		//Finds additional paths outside of nested dirs
		for _, userPath := range userPaths.Data {
			for _, interfacePath := range userPath.([]interface{}) {
				path := interfacePath.(string)
				if path != "" {
					foundService := false
					for _, service := range mod.VersionFilter {
						if (strings.HasSuffix(path, service) || strings.HasSuffix(path, service+"/")) && !foundService {
							foundService = true
							break
						}
					}

					if !foundService {
						continue
					}
					path = enginePath + "/" + path
					if mod.SubSectionName != "" {
						subSectionName := mod.SubSectionName
						if strings.HasPrefix(subSectionName, "/") {
							subSectionName = subSectionName[1:]
						}
						path = path + subSectionName
					}

					if _, ok := versionDataMap[path]; !ok {
						metadataValue, err := mod.ReadVersionMetadata(path, logger)
						if err != nil {
							logger.Println("Couldn't read version data at " + path)
						}
						if len(metadataValue) == 0 {
							continue
						}
						versionDataMap[path] = metadataValue
					}
				}
			}
		}
	}

	mod.Env = realEnv
	if len(versionDataMap) < 1 {
		return nil, fmt.Errorf("No version data available for this env")
	}
	return versionDataMap, nil
}

func (m *Modifier) recursivePathFinder(filePaths []string, versionDataMap map[string]map[string]interface{}, logger *log.Logger) {
	for _, filePath := range filePaths {
		foundService := false
		for _, service := range m.VersionFilter {
			if strings.Contains(filePath, service) && !foundService {
				foundService = true
			}
		}

		if !foundService {
			continue
		}

		subFilePaths, err := m.getPaths(filePath, logger)

		if len(subFilePaths) > 0 {
			m.recursivePathFinder(subFilePaths, versionDataMap, logger)
		}

		if err != nil {
			logger.Println(err.Error())
		}

		metadataValue, err := m.ReadVersionMetadata(filePath, logger)
		if len(metadataValue) == 0 {
			continue
		}
		versionDataMap[filePath] = metadataValue
	}
}

func (m *Modifier) getPaths(pathName string, logger *log.Logger) ([]string, error) {
	secrets, err := m.List(pathName, logger)
	//logger.Println("secrets " + pathName)
	//logger.Println(secrets)
	pathList := []string{}
	if err != nil {
		return nil, fmt.Errorf("Unable to list paths under %s in %s", pathName, m.Env)
	} else if secrets != nil {
		//add paths
		slicey := secrets.Data["keys"].([]interface{})
		//logger.Println("secrets are")
		//logger.Println(slicey)
		for _, pathEnd := range slicey {
			// skip local path if environment is not local
			if pathEnd != "local/" {
				//List is returning both pathEnd and pathEnd/
				path := pathName + pathEnd.(string)
				pathList = append(pathList, path)
			}
		}
		//logger.Println("pathList")
		//logger.Println(pathList)
		return pathList, nil
	}
	return pathList, nil
}
func (m *Modifier) GetTemplateFilePaths(pathName string, logger *log.Logger) ([]string, error) {
	secrets, err := m.List(pathName, logger)
	pathList := []string{}
	if err != nil {
		return nil, fmt.Errorf("Unable to list paths under %s in %s", pathName, m.Env)
	} else if secrets != nil {
		//add paths
		slicey := secrets.Data["keys"].([]interface{})

		for _, pathEnd := range slicey {
			//List is returning both pathEnd and pathEnd/
			path := pathName + pathEnd.(string)
			pathList = append(pathList, path)
		}

		subPathList := []string{}
		for _, path := range pathList {
			subsubList, _ := m.templateFileRecurse(path, logger)
			for _, subsub := range subsubList {
				//List is returning both pathEnd and pathEnd/
				subPathList = append(subPathList, subsub)
			}
		}
		if len(subPathList) != 0 {
			return subPathList, nil
		}
	}
	return pathList, nil
}
func (m *Modifier) templateFileRecurse(pathName string, logger *log.Logger) ([]string, error) {
	subPathList := []string{}
	subsecrets, err := m.List(pathName, logger)
	if err != nil {
		return subPathList, err
	} else if subsecrets != nil {
		subslice := subsecrets.Data["keys"].([]interface{})
		if subslice[0] != "template-file" {
			for _, pathEnd := range subslice {
				//List is returning both pathEnd and pathEnd/
				subpath := pathName + pathEnd.(string)
				subsublist, _ := m.templateFileRecurse(subpath, logger)
				if len(subsublist) != 0 {
					for _, subsub := range subsublist {
						//List is returning both pathEnd and pathEnd/
						subPathList = append(subPathList, subsub)
					}
				}
				subPathList = append(subPathList, subpath)
			}
		} else {
			subPathList = append(subPathList, pathName)
		}
	}
	return subPathList, nil
}

func getPathEnd(path string) string {
	strs := strings.Split(path, "/")
	for strs[len(strs)-1] == "" {
		strs = strs[:len(strs)-1]
	}
	return strs[len(strs)-1]
}

func (m *Modifier) ListSubsection(sectionKey string, project string, indexName string, logger *log.Logger) ([]string, error) {
	var indexes []string
	secret, err := m.List("super-secrets"+sectionKey+project+"/"+indexName, logger)
	if secret != nil {
		if _, ok := secret.Data["keys"].([]interface{}); ok {
			for _, index := range secret.Data["keys"].([]interface{}) {
				indexes = append(indexes, strings.TrimSuffix(index.(string), "/"))
			}
			return indexes, err
		}
	}
	return nil, errors.New("no regions were found")
}

// Given Project and Service, looks for a key index and returns it.
func (m *Modifier) FindIndexForService(project string, service string, logger *log.Logger) (string, error) {
	index := ""

	indexSecrets, err := m.List("super-secrets/Index/"+project, logger)
	if err != nil {
		return "", err
	}
	if indexSecrets != nil {
		indexValues := indexSecrets.Data["keys"].([]interface{})

		for _, indexValue := range indexValues {
			indexValueSecrets, valueErr := m.List("super-secrets/Index/"+project+"/"+indexValue.(string), logger)
			if valueErr != nil {
				continue
			}
			indexValues := indexValueSecrets.Data["keys"].([]interface{})

			subsectionValueSecrets, subsectionErr := m.List("super-secrets/Index/"+project+"/"+indexValue.(string)+"/"+indexValues[0].(string), logger)
			if subsectionErr != nil {
				continue
			}
			subsectionValues := subsectionValueSecrets.Data["keys"].([]interface{})

			for _, subSectionValue := range subsectionValues {
				if strings.TrimSuffix(subSectionValue.(string), "/") == service {
					index = strings.TrimSuffix(indexValue.(string), "/")
					goto indexFound
				}

			}
		}
	}
indexFound:

	return index, nil
}
