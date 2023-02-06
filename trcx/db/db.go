package db

import (
	"context"
	"io"
	"strings"
	"sync"

	"github.com/trimble-oss/tierceron/trcx/engine"
	eUtils "github.com/trimble-oss/tierceron/utils"
	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"

	sqle "github.com/dolthub/go-mysql-server"
	sqlememory "github.com/dolthub/go-mysql-server/memory"
	"github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/mysql_db"

	sqles "github.com/dolthub/go-mysql-server/sql"
)

var m sync.Mutex

// CreateEngine - creates a Tierceron query engine for query of configurations.
func CreateEngine(config *eUtils.DriverConfig,
	templatePaths []string, env string, dbname string) (*engine.TierceronEngine, error) {

	te := &engine.TierceronEngine{Database: sqlememory.NewDatabase(dbname), Engine: nil, TableCache: map[string]*engine.TierceronTable{}, Context: sqles.NewEmptyContext(), Config: *config}

	var goMod *helperkv.Modifier
	goMod, errModInit := helperkv.NewModifier(config.Insecure, config.Token, config.VaultAddress, config.Env, config.Regions, false, config.Log)
	if errModInit != nil {
		return nil, errModInit
	}
	goMod.Env = env
	/*	This is for versioning - used below
		projectServiceMap, err := goMod.GetProjectServicesMap()
		if err != nil {
			return nil, err
		}
	*/

	var envEnterprises []string
	goMod.Env = ""
	tempEnterprises, err := goMod.List("values", config.Log)
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		return nil, err
	}
	if tempEnterprises != nil {
		for _, enterprise := range tempEnterprises.Data["keys"].([]interface{}) {
			envEnterprises = append(envEnterprises, strings.Replace(enterprise.(string), "/", "", 1))
		}
		/* This is for versioning -> enhancements might be needed
			// Fun stuff here....
			var versionMetadata []string
			var wgEnterprise sync.WaitGroup
			// Load all vault table data into tierceron sql engine.
			for _, envEnterprise := range envEnterprises {
				wgEnterprise.Add(1)
				go func(config *eUtils.DriverConfig, enterpriseEnv string) {
					defer wgEnterprise.Done()
					if !strings.Contains(enterpriseEnv, ".") {
						return
					}

					tableMod, _, err := eUtils.InitVaultMod(config)
					if err != nil {
						eUtils.LogErrorMessage("Could not access vault.  Failure to start.", config.Log, false)
						return
					}

					tableMod.Env = ""
					versionMetadata = versionMetadata[:0]
					fileMetadata, err := tableMod.GetVersionValues(tableMod, config.WantCerts, "values/"+enterpriseEnv, config.Log)
					if fileMetadata == nil {
						return
					}
					if err != nil {
						eUtils.LogErrorObject(err, config.Log, false)
						return
					}

					var first map[string]interface{}
					for _, file := range fileMetadata {
						if first == nil {
							first = file
							break
						}
					}

					for versionNumber, _ := range first {
						versionMetadata = append(versionMetadata, versionNumber)
					}

					for _, versionNo := range versionMetadata {
						for project, services := range projectServiceMap {
							// TODO: optimize this for scale.
							for _, service := range services {
								for _, filter := range config.VersionFilter {
									if filter == service {
										TransformConfig(tableMod, te, enterpriseEnv, versionNo, project, service, config)
									}
								}
							}
						}
					}
				}(config, envEnterprise)
			}
			wgEnterprise.Wait()
		}
		*/
		te.Engine = sqle.NewDefault(sqlememory.NewMemoryDBProvider(te.Database))
		te.Engine.Analyzer.Debug = false
		te.Engine.Analyzer.Catalog.MySQLDb.SetPersister(&mysql_db.NoopPersister{})
	}
	if goMod != nil {
		goMod.Release()
	}

	return te, nil
}

// Query - queries configurations using standard ANSI SQL syntax.
// Example: select * from ServiceTechMobileAPI.configfile
func Query(te *engine.TierceronEngine, query string, queryLock *sync.Mutex) (string, []string, [][]interface{}, error) {
	// Create a test memory database and register it to the default engine.

	//ctx := sql.NewContext(context.Background(), sql.WithIndexRegistry(sql.NewIndexRegistry()), sql.WithViewRegistry(sql.NewViewRegistry())).WithCurrentDB(te.Database.Name())
	//ctx := sql.NewContext(context.Background()).WithCurrentDB(te.Database.Name())
	ctx := sqles.NewContext(context.Background())
	ctx.WithQuery(query)
	queryLock.Lock()
	//	te.Context = ctx
	schema, r, err := te.Engine.Query(ctx, query)
	queryLock.Unlock()
	if err != nil {
		return "", nil, nil, err
	}

	columns := []string{}
	matrix := [][]interface{}{}
	tableName := ""

	for _, col := range schema {
		if tableName == "" {
			tableName = col.Source
		}

		columns = append(columns, col.Name)
	}

	if len(columns) > 0 {
		// Iterate results and print them.
		okResult := false
		for {
			queryLock.Lock()
			row, err := r.Next(ctx)
			queryLock.Unlock()
			if err == io.EOF {
				break
			} else if err != nil {
				return "", nil, nil, err
			}
			rowData := []interface{}{}
			if len(columns) == 1 && columns[0] == "__ok_result__" { //This is for insert statements
				okResult = true
				if len(row) > 0 {
					if sqlOkResult, ok := row[0].(sqles.OkResult); ok {
						if sqlOkResult.RowsAffected > 0 {
							matrix = append(matrix, rowData)
						}
					}
				}
			} else {
				for _, col := range row {
					rowData = append(rowData, col)
				}
				matrix = append(matrix, rowData)
			}
		}
		if okResult {
			return "ok", nil, matrix, nil
		}
	}

	return tableName, columns, matrix, nil
}

// Query - queries configurations using standard ANSI SQL syntax.
// Example: select * from ServiceTechMobileAPI.configfile
func QueryWithBindings(te *engine.TierceronEngine, query string, bindings map[string]sql.Expression, queryLock *sync.Mutex) (string, []string, [][]interface{}, error) {
	// Create a test memory database and register it to the default engine.

	//ctx := sql.NewContext(context.Background(), sql.WithIndexRegistry(sql.NewIndexRegistry()), sql.WithViewRegistry(sql.NewViewRegistry())).WithCurrentDB(te.Database.Name())
	//ctx := sql.NewContext(context.Background()).WithCurrentDB(te.Database.Name())
	ctx := sql.NewContext(context.Background())
	ctx.WithQuery(query)
	queryLock.Lock()
	//	te.Context = ctx
	schema, r, queryErr := te.Engine.QueryWithBindings(ctx, query, bindings)
	queryLock.Unlock()
	if queryErr != nil {
		return "", nil, nil, queryErr
	}

	columns := []string{}
	matrix := [][]interface{}{}
	tableName := ""

	for _, col := range schema {
		if tableName == "" {
			tableName = col.Source
		}

		columns = append(columns, col.Name)
	}

	if len(columns) > 0 {
		// Iterate results and print them.
		okResult := false
		for {
			queryLock.Lock()
			row, err := r.Next(ctx)
			queryLock.Unlock()
			if err == io.EOF {
				break
			}
			rowData := []interface{}{}
			if len(columns) == 1 && columns[0] == "__ok_result__" { //This is for insert statements
				okResult = true
				if len(row) > 0 {
					if sqlOkResult, ok := row[0].(sql.OkResult); ok {
						if sqlOkResult.RowsAffected > 0 {
							matrix = append(matrix, rowData)
						}
					}
				}
			} else {
				for _, col := range row {
					rowData = append(rowData, col.(string))
				}
				matrix = append(matrix, rowData)
			}
		}
		if okResult {
			return "ok", nil, matrix, nil
		}
	}

	return tableName, columns, matrix, nil
}
