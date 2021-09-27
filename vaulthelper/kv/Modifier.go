package kv

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

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
	httpClient           *http.Client // Handle to http client.
	client               *api.Client  // Client connected to vault
	logical              *api.Logical // Logical used for read/write options
	Env                  string       // Environment (local/dev/QA; Initialized to secrets)
	Regions              []string     // Supported regions
	SecretDictionary     *api.Secret  // Current Secret Dictionary Cache.
	Version              string       // Version for data
	ProjectVersionFilter []string     // Used to filter vault paths
}

// NewModifier Constructs a new modifier struct and connects to the vault
// @param token 	The access token needed to connect to the vault
// @param address	The address of the API endpoint for the server
// @param env   	The environment currently connecting to.
// @return 			A pointer to the newly contstructed modifier object (Note: path set to default),
// 		   			Any errors generated in creating the client
func NewModifier(insecure bool, token string, address string, env string, regions []string) (*Modifier, error) {
	if len(address) == 0 {
		address = "http://127.0.0.1:8020" // Default address
	}
	httpClient, err := CreateHTTPClient(insecure, address, env)
	if err != nil {
		return nil, err
	}
	// Create client
	modClient, err := api.NewClient(&api.Config{
		Address:    address,
		HttpClient: httpClient,
	})
	if err != nil {
		fmt.Println("vaultHost: " + modClient.Address())
		return nil, err
	}

	// Set access token and path for this modifier
	modClient.SetToken(token)

	// Return the modifier
	return &Modifier{httpClient: httpClient, client: modClient, logical: modClient.Logical(), Env: "secret", Regions: regions, Version: ""}, nil
}

// ValidateEnvironment Ensures token has access to requested data.
func (m *Modifier) ValidateEnvironment(environment string, init bool) bool {
	if strings.Contains(environment, "local") {
		environment = "local"
	}
	desiredPolicy := "config_" + strings.ToLower(environment)

	if init {
		desiredPolicy = "vault_pub_" + strings.ToLower(environment)
	}

	secret, err := m.client.Auth().Token().LookupSelf()

	if err != nil {
		fmt.Printf("LookupSelf Auth failure: %v\n", err)
		if strings.Contains(err.Error(), "x509: certificate") {
			os.Exit(-1)
		}
	}

	valid := false
	if err == nil {
		policies, _ := secret.TokenPolicies()

		for _, policy := range policies {
			if policy == "root" {
				valid = true
			}
			if strings.ToLower(policy) == desiredPolicy {
				valid = true
			}
		}

	}
	return valid
}

// Writes the key,value pairs in data to the vault
//
// @param   data A set of key,value pairs to be written
//
// @return	Warnings (if any) generated from the vault,
//			errors generated by writing
func (m *Modifier) Write(path string, data map[string]interface{}) ([]string, error) {
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
	if len(pathBlocks) > 1 {
		fullPath += pathBlocks[1]
	}
	Secret, err := m.logical.Write(fullPath, sendData)

	if Secret == nil { // No warnings
		return nil, err
	}
	return Secret.Warnings, err
}

// ReadData Reads the most recent data from the path referenced by this Modifier
// @return	A Secret pointer that contains key,value pairs and metadata
//			errors generated from reading
func (m *Modifier) ReadData(path string) (map[string]interface{}, error) {
	// Create full path
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

	var versionMap = make(map[string][]string)
	var secret *api.Secret
	var err error
	if m.Version != "" && !strings.HasPrefix(path, "templates") {
		versionSlice := []string{m.Version}
		versionMap["version"] = versionSlice
		secret, err = m.logical.ReadWithData(fullPath, versionMap)
	} else {
		secret, err = m.logical.Read(fullPath)
	}

	if secret == nil {
		return nil, err
	}
	if data, ok := secret.Data["data"].(map[string]interface{}); ok {
		return data, err
	}
	return nil, errors.New("Could not get data from vault response")
}

//ReadMapValue takes a valueMap, path, and a key and returns the corresponding value from the vault
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

//ReadValue takes a path and a key and returns the corresponding value from the vault
func (m *Modifier) ReadValue(path string, key string) (string, error) {
	valueMap, err := m.ReadData(path)
	if err != nil {
		return "", err
	}
	return m.ReadMapValue(valueMap, path, key)
}

// ReadMetadata Reads the Metadata from the path referenced by this Modifier
// @return	A Secret pointer that contains key,value pairs and metadata
//			errors generated from reading
func (m *Modifier) ReadMetadata(path string) (map[string]interface{}, error) {
	pathBlocks := strings.SplitAfterN(path, "/", 2)
	fullPath := pathBlocks[0] + "data/"
	if !noEnvironments[pathBlocks[0]] {
		fullPath += m.Env + "/"
	}
	fullPath += pathBlocks[1]
	secret, err := m.logical.Read(fullPath)
	if data, ok := secret.Data["metadata"].(map[string]interface{}); ok {
		return data, err
	}
	return nil, errors.New("Could not get metadata from vault response")
}

//ReadTemplateVersions Reads the Metadata of all versions from the path referenced by this Modifier
func (m *Modifier) ReadTemplateVersions(path string) (map[string]interface{}, error) {
	// Create full path
	pathBlocks := strings.SplitAfterN(path, "/", 2)
	fullPath := pathBlocks[0] + "metadata/"

	if !noEnvironments[pathBlocks[0]] {
		fullPath += m.Env + "/"
	}
	if len(pathBlocks) > 1 {
		fullPath += pathBlocks[1]
	}
	secret, err := m.logical.Read(fullPath)
	if secret == nil {
		return nil, err
	}
	if versionsData, ok := secret.Data["versions"].(map[string]interface{}); ok {
		return versionsData, err
	}
	return nil, errors.New("Could not get metadata of versions from vault response")
}

//List lists the paths underneath this one
func (m *Modifier) List(path string) (*api.Secret, error) {
	pathBlocks := strings.SplitAfterN(path, "/", 2)
	if len(pathBlocks) == 1 {
		pathBlocks[0] += "/"
	}

	fullPath := pathBlocks[0] + "metadata/"
	if !noEnvironments[pathBlocks[0]] {
		fullPath += m.Env + "/"
	} else if strings.HasPrefix(m.Env, "local") { //if local environment, add env to fullpath
		fullPath += m.Env + "/"
	}
	if len(pathBlocks) > 1 {
		fullPath += pathBlocks[1]
	}
	return m.logical.List(fullPath)
}

//AdjustValue adjusts the value at the given path/key by n
func (m *Modifier) AdjustValue(path string, key string, n int) ([]string, error) {
	// Get the existing data at the path
	oldData, err := m.ReadData(path)
	if err != nil {
		return nil, err
	}
	if oldData == nil { // Path has not been used yet, create an empty map
		oldData = make(map[string]interface{})
	}
	// Try to fetch the value with the given key, start empty values with 0
	if oldData[key] == nil {
		oldData[key] = "0"
	}
	// Convert from stored string value to int
	oldValue, err := strconv.Atoi(oldData[key].(string))
	if err != nil {
		return []string{"Could not convert value to int at: " + key}, err
	}
	newValue := strconv.Itoa(oldValue + n)
	oldData[key] = newValue
	return m.Write(path, oldData)
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

//GetVersionValues gets filepath for values and grabs metadata for those paths.
func (m *Modifier) GetVersionValues(mod *Modifier, enginePath string) (map[string]map[string]interface{}, error) {
	envCheck := strings.Split(mod.Env, "_")
	mod.Env = envCheck[0]
	userPaths, err := mod.List(enginePath + "/")
	versionDataMap := make(map[string]map[string]interface{}, 0)
	//data := make([]string, 0)
	if err != nil {
		return nil, err
	}
	if userPaths == nil {
		return nil, err
	}

	//Finds additional paths outside of nested dirs
	for _, userPath := range userPaths.Data {
		for _, interfacePath := range userPath.([]interface{}) {
			path := interfacePath.(string)
			if path != "" {
				path = enginePath + "/" + path
				metadataValue, err := mod.ReadTemplateVersions(path)
				if err != nil {
					fmt.Println("Couldn't read version data at " + path)
				}
				if len(metadataValue) == 0 {
					continue
				}
				versionDataMap[path] = metadataValue
			}
		}
	}

	//get a list of projects under values
	projectPaths, err := getPaths(mod, enginePath+"/")
	if err != nil {
		return nil, err
	}

	for _, projectPath := range projectPaths {
		//get a list of files under project
		servicePaths, err := getPaths(mod, projectPath)
		//fmt.Println("servicePaths")
		//fmt.Println(servicePaths)
		if err != nil {
			return nil, err
		}

		if len(projectPaths) > 0 {
			recursivePathFinder(mod, servicePaths, versionDataMap)
		}
		metadataValue, err := mod.ReadTemplateVersions(projectPath)
		if err != nil {
			err := fmt.Errorf("Unable to fetch data from %s", projectPath)
			return nil, err
		}
		if len(metadataValue) == 0 {
			continue
		}
		versionDataMap[projectPath] = metadataValue

		for _, servicePath := range servicePaths {
			if !strings.Contains(projectPath, mod.ProjectVersionFilter[0]) {
				continue
			}
			//get a list of files under project
			filePaths, err := getPaths(mod, servicePath)
			if err != nil {
				return nil, err
			}

			if len(servicePaths) > 0 {
				recursivePathFinder(mod, servicePaths, versionDataMap)
			}
			metadataValue, err := mod.ReadTemplateVersions(servicePath)
			if err != nil {
				err := fmt.Errorf("Unable to fetch data from %s", servicePath)
				return nil, err
			}
			if len(metadataValue) == 0 {
				continue
			}
			versionDataMap[servicePath] = metadataValue

			for _, filePath := range filePaths {
				subFilePaths, err := getPaths(mod, filePath)
				//get a list of values
				if len(subFilePaths) > 0 {
					recursivePathFinder(mod, subFilePaths, versionDataMap)
				}
				metadataValue, err := mod.ReadTemplateVersions(filePath)
				if err != nil {
					err := fmt.Errorf("Unable to fetch data from %s", filePath)
					return nil, err
				}
				if len(metadataValue) == 0 {
					continue
				}
				versionDataMap[filePath] = metadataValue
			}
		}
	}
	return versionDataMap, nil
}

func recursivePathFinder(mod *Modifier, filePaths []string, versionDataMap map[string]map[string]interface{}) {
	for _, filePath := range filePaths {
		if !strings.Contains(filePath, mod.ProjectVersionFilter[0]) {
			continue
		}

		subFilePaths, err := getPaths(mod, filePath)

		if len(subFilePaths) > 0 {
			recursivePathFinder(mod, subFilePaths, versionDataMap)
		}

		if err != nil {
			fmt.Println(err)
		}

		metadataValue, err := mod.ReadTemplateVersions(filePath)
		if len(metadataValue) == 0 {
			continue
		}
		versionDataMap[filePath] = metadataValue
	}
}

func getPaths(mod *Modifier, pathName string) ([]string, error) {
	secrets, err := mod.List(pathName)
	//fmt.Println("secrets " + pathName)
	//fmt.Println(secrets)
	pathList := []string{}
	if err != nil {
		return nil, fmt.Errorf("Unable to list paths under %s in %s", pathName, mod.Env)
	} else if secrets != nil {
		//add paths
		slicey := secrets.Data["keys"].([]interface{})
		//fmt.Println("secrets are")
		//fmt.Println(slicey)
		for _, pathEnd := range slicey {
			// skip local path if environment is not local
			if pathEnd != "local/" {
				//List is returning both pathEnd and pathEnd/
				path := pathName + pathEnd.(string)
				pathList = append(pathList, path)
			}
		}
		//fmt.Println("pathList")
		//fmt.Println(pathList)
		return pathList, nil
	}
	return pathList, nil
}
func getTemplateFilePaths(mod *Modifier, pathName string) ([]string, error) {
	secrets, err := mod.List(pathName)
	pathList := []string{}
	if err != nil {
		return nil, fmt.Errorf("Unable to list paths under %s in %s", pathName, mod.Env)
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
			subsubList, _ := templateFileRecurse(mod, path)
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
func templateFileRecurse(mod *Modifier, pathName string) ([]string, error) {
	subPathList := []string{}
	subsecrets, err := mod.List(pathName)
	if err != nil {
		return subPathList, err
	} else if subsecrets != nil {
		subslice := subsecrets.Data["keys"].([]interface{})
		if subslice[0] != "template-file" {
			for _, pathEnd := range subslice {
				//List is returning both pathEnd and pathEnd/
				subpath := pathName + pathEnd.(string)
				subsublist, _ := templateFileRecurse(mod, subpath)
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
