package db

import (
	"context"
	"fmt"
	"io"
	"strings"

	vcutils "tierceron/trcconfig/utils"
	"tierceron/trcx/extract"
	eUtils "tierceron/utils"
	"tierceron/vaulthelper/kv"

	sqle "github.com/dolthub/go-mysql-server"
	"github.com/dolthub/go-mysql-server/memory"
	"github.com/dolthub/go-mysql-server/sql"
)

// CreateEngine - creates a Tierceron query engine for query of configurations.
func CreateEngine(config eUtils.DriverConfig,
	templatePaths []string) *TierceronEngine {
	var goMod *kv.Modifier

	envVersion := strings.Split(config.Env, "_")
	if len(envVersion) != 2 {
		// Make it so.
		config.Env = config.Env + "_0"
		envVersion = strings.Split(config.Env, "_")
	}

	env := envVersion[0]
	version := envVersion[1]

	if config.Token != "" {
		var err error
		goMod, err = kv.NewModifier(config.Insecure, config.Token, config.VaultAddress, config.Env, config.Regions)
		if err != nil {
			panic(err)
		}
		goMod.Env = env
		goMod.Version = version
	}

	db := memory.NewDatabase(env)

	// var cds *vcutils.ConfigDataStore
	// if goMod != nil {
	// 	cds = new(vcutils.ConfigDataStore)
	// }

	projectServiceMap, err := goMod.GetProjectServicesMap()
	if err != nil {
		return nil
	}

	for project, services := range projectServiceMap {
		for _, service := range services {
			listPath := "templates/" + project + "/" + service
			secret, err := goMod.List(listPath)
			if err != nil {
				return nil
			}
			templatePaths := []string{}
			for _, fileName := range secret.Data["keys"].([]interface{}) {
				if strFile, ok := fileName.(string); ok {
					if strFile[len(strFile)-1] != '/' { // Skip subdirectories where template files are stored
						templatePaths = append(templatePaths, listPath+"/"+strFile)
					} else {
						templatePaths = append(templatePaths, listPath+"/"+strings.ReplaceAll(strFile, "/", ""))
					}
				}
			}

			// TODO: Make this async for performance...
			for _, templatePath := range templatePaths {

				var templateResult extract.TemplateResultData
				templateResult.ValueSection = map[string]map[string]map[string]string{}
				templateResult.ValueSection["values"] = map[string]map[string]string{}

				templateResult.SecretSection = map[string]map[string]map[string]string{}
				templateResult.SecretSection["super-secrets"] = map[string]map[string]string{}

				var cds *vcutils.ConfigDataStore
				if goMod != nil {
					cds = new(vcutils.ConfigDataStore)
					cds.Init(goMod, config.SecretMode, true, project, service)
				}

				_, _, _, templateResult.TemplateDepth = extract.ToSeed(goMod,
					cds,
					templatePath,
					config.Log,
					project,
					service,
					true,
					&(templateResult.InterfaceTemplateSection),
					&(templateResult.ValueSection),
					&(templateResult.SecretSection),
				)

				//
				// What we need is in ValueSection and SecretSection...
				//

				// Create tables with naming convention: Service.configFileName  Column names should be template variable names.
				table := memory.NewTable("tableName", sql.Schema{
					{Name: "col1", Type: sql.Text, Source: "tableName"},
					{Name: "col2", Type: sql.Text, Source: "tableName2"},
				})
				db.AddTable("tableName", table)
				ctx := sql.NewEmptyContext()

				rows := []sql.Row{
					sql.NewRow("col1", "col2"),
					sql.NewRow("col1", "col2"),
				}

				for _, row := range rows {
					table.Insert(ctx, row)
				}
			}

		}

	}

	e := sqle.NewDefault(sql.NewDatabaseProvider(db))
	te := TierceronEngine{Name: db.Name(), Engine: e}

	return &te
}

// Query - queries configurations using standard ANSI SQL syntax.
// Example: select * from ServiceTechMobileAPI.configfile
func Query(te *TierceronEngine, query string) {
	// Create a test memory database and register it to the default engine.

	ctx := sql.NewContext(context.Background(), sql.WithIndexRegistry(sql.NewIndexRegistry()), sql.WithViewRegistry(sql.NewViewRegistry())).WithCurrentDB(te.Name)

	_, r, err := te.Engine.Query(ctx, query)
	if err != nil {

	}

	// Iterate results and print them.
	for {
		row, err := r.Next()
		if err == io.EOF {
			break
		}

		name := row[0]
		count := row[1]

		fmt.Println(name, count)
	}

	// Output: John Doe 2
}
