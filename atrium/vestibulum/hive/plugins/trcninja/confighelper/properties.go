package confighelper

// import "github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase/utils"

// Properties stores all configuration properties for a project.
type Properties struct {
	AuthEndpoint string
}

func NewPluginProperties(
	tokenNamePtr *string,
	env string,
	authenv string,
	project string,
	commonPaths []string,
	servicesWanted ...string) (*Properties, error) {
	properties := Properties{}
	if len(*tokenNamePtr) > 0 {
		// properties.mod.Env = env

		// properties.cds = new(utils.ConfigDataStore)
		// properties.cds.Init(properties.mod, true, true, project, commonPaths, servicesWanted...)
		// if initErr != nil {
		// 	config.Log.Println(initErr)
		// }
		// config.Log.Println("Finished creating properties")
	}

	return &properties, nil
}

// GetValue gets an invididual configuration value for a service from the data store.
// func (p *Properties) GetValue(service string, keyPath []string, key string) (string, error) {
// 	return p.cds.GetValue(service, keyPath, key)
// }

// // GetConfigValue gets an invididual configuration value for a service from the data store.
// func (p *Properties) GetConfigValue(service string, config string, key string) (string, bool) {
// 	return p.cds.GetConfigValue(service, config, key)
// }

// // GetConfigValues gets an invididual configuration value for a service from the data store.
// func (p *Properties) GetConfigValues(service string, config string) (map[string]interface{}, bool) {
// 	return p.cds.GetConfigValues(service, config)
// }
