package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/trimble-oss/tierceron/pkg/utils/config"

	bitcore "github.com/trimble-oss/tierceron-core/v2/bitlock"
	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	"github.com/trimble-oss/tierceron/atrium/trcdb/engine"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"

	sqle "github.com/dolthub/go-mysql-server"
	sqlememory "github.com/dolthub/go-mysql-server/memory"
	"github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/mysql_db"

	sqles "github.com/dolthub/go-mysql-server/sql"
)

var m sync.Mutex

// isCreateTableStatement checks if a SQL query is a CREATE TABLE statement
func isCreateTableStatement(query string) bool {
	trimmedQuery := strings.TrimSpace(query)
	return strings.HasPrefix(strings.ToUpper(trimmedQuery), "CREATE TABLE")
}

// ColumnDef represents a column definition in a table
type ColumnDef struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// TableSchema represents the schema of a table
type TableSchema struct {
	TableName string      `json:"table_name"`
	Columns   []ColumnDef `json:"columns"`
	CreatedAt string      `json:"created_at"`
}

// parseCreateTableStatement extracts table name and column definitions from a CREATE TABLE statement
// Returns (tableName, columns, error)
func parseCreateTableStatement(query string) (string, []ColumnDef, error) {
	// Regex to match: CREATE TABLE [IF NOT EXISTS] tablename (column definitions)
	createTableRegex := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:[\w]+\.)?(\w+)\s*\((.*)\)`)
	matches := createTableRegex.FindStringSubmatch(query)
	if len(matches) < 3 {
		return "", nil, errors.New("invalid CREATE TABLE syntax")
	}

	tableName := matches[1]
	columnDefs := matches[2]

	// Parse column definitions
	var columns []ColumnDef

	// Split by comma, but be careful with nested parentheses
	columnParts := strings.Split(columnDefs, ",")
	for _, part := range columnParts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Skip constraint definitions (PRIMARY KEY, UNIQUE, KEY, INDEX, etc.)
		upperPart := strings.ToUpper(part)
		if strings.HasPrefix(upperPart, "PRIMARY KEY") ||
			strings.HasPrefix(upperPart, "UNIQUE") ||
			strings.HasPrefix(upperPart, "KEY ") ||
			strings.HasPrefix(upperPart, "INDEX") ||
			strings.HasPrefix(upperPart, "FOREIGN KEY") ||
			strings.HasPrefix(upperPart, "CONSTRAINT") {
			continue
		}

		// Extract column name and type
		fields := strings.Fields(part)
		if len(fields) >= 2 {
			colName := fields[0]
			colType := fields[1]

			// Validate that column type is string-like (VARCHAR, CHAR, TEXT, STRING, etc.)
			upperColType := strings.ToUpper(colType)
			if !isStringType(upperColType) {
				return "", nil, fmt.Errorf("column %s has type %s; only string types (VARCHAR, CHAR, TEXT, etc.) are allowed", colName, colType)
			}

			columns = append(columns, ColumnDef{
				Name: colName,
				Type: colType,
			})
		}
	}

	if len(columns) == 0 {
		return "", nil, errors.New("no valid columns found in CREATE TABLE statement")
	}

	return tableName, columns, nil
}

// isStringType checks if a column type is a string type
func isStringType(colType string) bool {
	stringTypes := map[string]bool{
		"VARCHAR":    true,
		"CHAR":       true,
		"TEXT":       true,
		"STRING":     true,
		"TINYTEXT":   true,
		"MEDIUMTEXT": true,
		"LONGTEXT":   true,
	}
	return stringTypes[colType]
}

// buildSchemaTemplate creates a schema template in JSON format for Vault storage
func buildSchemaTemplate(tableName string, columns []ColumnDef) ([]byte, error) {
	schema := TableSchema{
		TableName: tableName,
		Columns:   columns,
		CreatedAt: fmt.Sprintf("%d", time.Now().Unix()),
	}

	schemaBytes, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, err
	}

	return schemaBytes, nil
}

// handleCreateTableStatement processes a CREATE TABLE statement
// It validates the controller database, parses the statement, pushes schema to Vault,
// and registers the table with the TierceronEngine
// Note: This function should only be called if IsCreateTableEnabled() returns true
func handleCreateTableStatement(te *engine.TierceronEngine, query string) error {
	// Only allow CREATE TABLE in the controller database
	if !isControllerDatabase(te.Database.Name()) {
		return fmt.Errorf("CREATE TABLE is only allowed in the controller database, not in %s", te.Database.Name())
	}

	// Parse the CREATE TABLE statement
	tableName, columns, err := parseCreateTableStatement(query)
	if err != nil {
		return fmt.Errorf("failed to parse CREATE TABLE statement: %w", err)
	}

	// Build schema template
	schemaBytes, err := buildSchemaTemplate(tableName, columns)
	if err != nil {
		return fmt.Errorf("failed to build schema template: %w", err)
	}

	// Get a Vault modifier to push the schema to Vault
	mod, err := helperkv.NewModifierFromCoreConfig(te.Config.CoreConfig,
		"config_token_"+te.Config.CoreConfig.Env,
		te.Config.CoreConfig.Env,
		false)
	if err != nil {
		return fmt.Errorf("failed to create vault modifier: %w", err)
	}
	defer mod.Release()

	// Push schema to Vault at templates/settings/{tableName}/schema
	templatePath := fmt.Sprintf("templates/settings/%s/schema", tableName)
	warn, err := mod.Write(templatePath, map[string]any{
		"data": schemaBytes,
		"ext":  ".json",
	}, te.Config.CoreConfig.Log)

	if err != nil || len(warn) > 0 {
		return fmt.Errorf("failed to push schema to vault: %w, warnings: %v", err, warn)
	}

	if te.Config.CoreConfig.Log != nil {
		te.Config.CoreConfig.Log.Printf("Successfully pushed schema for table %s to vault at %s\n", tableName, templatePath)
	}

	// Note: The actual table creation in the MySQL engine happens in the Query functions
	// after this handler returns successfully. If this returns an error, the CREATE TABLE
	// statement will not be executed in the database.

	return nil
}

// isControllerDatabase checks if the given database name is the controller database
// The controller database is always "TierceronFlow" as defined in tierceron-core
func isControllerDatabase(dbName string) bool {
	return dbName == flowcore.TierceronControllerFlow.FlowName()
}

// registerFlowInTierceronFlow inserts a new flow definition into the TierceronFlow table
// and initializes it for flow processing using the generic flow pattern.
// tfmContextI is passed as 'any' to avoid circular imports (it's TrcFlowMachineContext from trcflow/core)
// logger is passed as 'any' and should have a Printf method (typically *log.Logger or similar)
func registerFlowInTierceronFlow(tfmContextI any, tableName string, logger any) {
	if tfmContextI == nil || tableName == "" {
		return
	}

	// Use reflection to call methods on TrcFlowMachineContext without direct import
	tfmContextVal := reflect.ValueOf(tfmContextI)
	if !tfmContextVal.IsValid() {
		return
	}

	// Get the GetFlowContext method and call it with TierceronControllerFlow
	getFlowContextMethod := tfmContextVal.MethodByName("GetFlowContext")
	if !getFlowContextMethod.IsValid() {
		logMessage(logger, fmt.Sprintf("Could not find GetFlowContext method for registering flow: %s\n", tableName))
		return
	}

	// Call GetFlowContext with the controller flow name
	controllerFlowName := flowcore.TierceronControllerFlow.FlowName()
	result := getFlowContextMethod.Call([]reflect.Value{reflect.ValueOf(flowcore.FlowNameType(controllerFlowName))})
	if len(result) == 0 || result[0].IsNil() {
		logMessage(logger, fmt.Sprintf("Could not get TierceronFlow context for registering new flow: %s\n", tableName))
		return
	}

	tfContext := result[0].Interface()

	// Insert row into TierceronFlow with the new flow definition
	// Use INSERT IGNORE to silently ignore duplicate key errors (no-op if flow already exists)
	// Default state=0, syncMode=nosync, syncFilter=, flowAlias=tableName
	insertQuery := fmt.Sprintf(
		"INSERT IGNORE INTO %s.%s (flowName, state, syncMode, syncFilter, flowAlias, lastModified) VALUES ('%s', 0, 'nosync', '', '%s', NOW())",
		controllerFlowName,
		flowcore.TierceronControllerFlow.TableName(),
		tableName,
		tableName,
	)

	// Call CallDBQuery method
	callDBQueryMethod := tfmContextVal.MethodByName("CallDBQuery")
	if !callDBQueryMethod.IsValid() {
		logMessage(logger, fmt.Sprintf("Failed to find CallDBQuery method for registering flow: %s\n", tableName))
		return
	}

	queryMap := map[string]any{"TrcQuery": insertQuery}
	flowNames := []flowcore.FlowNameType{flowcore.FlowNameType(controllerFlowName)}

	result = callDBQueryMethod.Call([]reflect.Value{
		reflect.ValueOf(tfContext),
		reflect.ValueOf(queryMap),
		reflect.ValueOf(nil),
		reflect.ValueOf(false),
		reflect.ValueOf("INSERT"),
		reflect.ValueOf(flowNames),
		reflect.ValueOf(""),
	})

	if len(result) > 1 {
		errResult := result[1].Interface()
		if errResult != nil {
			logMessage(logger, fmt.Sprintf("Failed to insert flow definition for %s into TierceronFlow: %v\n", tableName, errResult))
			return
		}
	}

	logMessage(logger, fmt.Sprintf("Successfully registered new flow '%s' in TierceronFlow table\n", tableName))
}

// logMessage logs a message using reflection to avoid type assertions
// logger should have a Printf(string, ...interface{}) method
func logMessage(logger any, message string) {
	if logger == nil {
		return
	}

	logVal := reflect.ValueOf(logger)
	printfMethod := logVal.MethodByName("Printf")
	if printfMethod.IsValid() {
		printfMethod.Call([]reflect.Value{reflect.ValueOf(message)})
	}
}

// HandleCreateTableTemplate is the public entry point for CREATE TABLE template generation.
// This is called from the query callback after a CREATE TABLE statement has been executed.
// It parses the CREATE TABLE statement, generates a template in Vault, and registers the flow.
// Note: This does NOT execute the CREATE TABLE - that's done by the normal query flow.
func HandleCreateTableTemplate(te *engine.TierceronEngine, query string, tfmContext any) {
	if te == nil || te.Config.CoreConfig == nil {
		return
	}

	// Only generate template for controller database
	if !isControllerDatabase(te.Database.Name()) {
		return
	}

	// Parse the CREATE TABLE statement
	tableName, columns, err := parseCreateTableStatement(query)
	if err != nil {
		te.Config.CoreConfig.Log.Printf("Failed to parse CREATE TABLE statement: %v\n", err)
		return
	}

	// Build schema template
	schemaBytes, err := buildSchemaTemplate(tableName, columns)
	if err != nil {
		te.Config.CoreConfig.Log.Printf("Failed to build schema template: %v\n", err)
		return
	}

	// Get a Vault modifier to push the schema to Vault
	mod, err := helperkv.NewModifierFromCoreConfig(te.Config.CoreConfig,
		"config_token_"+te.Config.CoreConfig.Env,
		te.Config.CoreConfig.Env,
		false)
	if err != nil {
		te.Config.CoreConfig.Log.Printf("Failed to create vault modifier: %v\n", err)
		return
	}
	defer mod.Release()

	// Push schema to Vault at templates/settings/{tableName}/schema
	templatePath := fmt.Sprintf("templates/settings/%s/schema", tableName)
	warn, err := mod.Write(templatePath, map[string]any{
		"data": schemaBytes,
		"ext":  ".json",
	}, te.Config.CoreConfig.Log)

	if err != nil || len(warn) > 0 {
		te.Config.CoreConfig.Log.Printf("Failed to push schema to vault: %v, warnings: %v\n", err, warn)
		return
	}

	te.Config.CoreConfig.Log.Printf("Successfully pushed schema for table %s to vault at %s\n", tableName, templatePath)

	// Register the new flow in TierceronFlow table
	// This creates a flow definition that can be started and managed
	if tfmContext != nil {
		registerFlowInTierceronFlow(tfmContext, tableName, te.Config.CoreConfig.Log)
	}
}

// CreateEngine - creates a Tierceron query engine for query of configurations.
func CreateEngine(driverConfig *config.DriverConfig,
	templatePaths []string, env string, dbname string) (*engine.TierceronEngine, error) {

	te := &engine.TierceronEngine{Database: sqlememory.NewDatabase(dbname), Engine: nil, TableCache: map[string]*engine.TierceronTable{}, Context: sqles.NewEmptyContext(), Config: *driverConfig}

	var goMod *helperkv.Modifier
	tokenNamePtr := driverConfig.CoreConfig.GetCurrentToken("config_token_%s")
	goMod, errModInit := helperkv.NewModifierFromCoreConfig(driverConfig.CoreConfig, *tokenNamePtr, driverConfig.CoreConfig.Env, false)
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
	tempEnterprises, err := goMod.List("values", driverConfig.CoreConfig.Log)
	if err != nil {
		eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
		return nil, err
	}
	if tempEnterprises != nil {
		for _, enterprise := range tempEnterprises.Data["keys"].([]any) {
			envEnterprises = append(envEnterprises, strings.Replace(enterprise.(string), "/", "", 1))
		}
		/* This is for versioning -> enhancements might be needed
			// Fun stuff here....
			var versionMetadata []string
			var wgEnterprise sync.WaitGroup
			// Load all vault table data into tierceron sql engine.
			for _, envEnterprise := range envEnterprises {
				wgEnterprise.Add(1)
				go func(driverConfig *config.DriverConfig, enterpriseEnv string) {
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

					var first map[string]any
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
func Query(te *engine.TierceronEngine, query string, queryLock *sync.Mutex) (string, []string, [][]any, error) {
	// Create a test memory database and register it to the default engine.

	if strings.Contains(query, "%s.") {
		query = fmt.Sprintf(query, te.Database.Name())
	}
	//ctx := sql.NewContext(context.Background(), sql.WithIndexRegistry(sql.NewIndexRegistry()), sql.WithViewRegistry(sql.NewViewRegistry())).WithCurrentDB(te.Database.Name())
	//ctx := sql.NewContext(context.Background()).WithCurrentDB(te.Database.Name())
	ctx := sqles.NewContext(context.Background())
	ctx.WithQuery(query)
	queryLock.Lock()
	//	te.Context = ctx
	schema, r, err := te.Engine.Query(ctx, query)
	queryLock.Unlock()
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return "", nil, nil, errors.New("Duplicate primary key found.")
		}
		return "", nil, nil, err
	}

	columns := []string{}
	matrix := [][]any{}
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
			rowData := []any{}
			if sqles.IsOkResult(row) { //This is for insert statements
				okResult = true
				sqlOkResult := sqles.GetOkResult(row)
				if sqlOkResult.RowsAffected > 0 {
					matrix = append(matrix, rowData)
				} else {
					if sqlOkResult.InsertID > 0 {
						rowData = append(rowData, sqlOkResult.InsertID)
						matrix = append(matrix, rowData)
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
// Able to run query with multiple flows
// Example: select * from ServiceTechMobileAPI.configfile
func QueryN(te *engine.TierceronEngine, query string, queryMask uint64, bitlock bitcore.BitLock) (string, []string, [][]any, error) {
	// Create a test memory database and register it to the default engine.

	for _, literal := range []string{"from %s.", "FROM %s.", "join %s.", "JOIN %s."} {
		if strings.Contains(query, literal) {
			query = strings.ReplaceAll(query, literal, fmt.Sprintf(literal, te.Database.Name()))
		}
	}
	//ctx := sql.NewContext(context.Background(), sql.WithIndexRegistry(sql.NewIndexRegistry()), sql.WithViewRegistry(sql.NewViewRegistry())).WithCurrentDB(te.Database.Name())
	//ctx := sql.NewContext(context.Background()).WithCurrentDB(te.Database.Name())
	ctx := sqles.NewContext(context.Background())
	ctx.WithQuery(query)
	bitlock.Lock(queryMask)
	//	te.Context = ctx
	schema, r, err := te.Engine.Query(ctx, query)
	bitlock.Unlock(queryMask)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return "", nil, nil, errors.New("Duplicate primary key found.")
		}
		return "", nil, nil, err
	}

	columns := []string{}
	matrix := [][]any{}
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
			bitlock.Lock(queryMask)
			row, err := r.Next(ctx)
			bitlock.Unlock(queryMask)
			if err == io.EOF {
				break
			} else if err != nil {
				return "", nil, nil, err
			}
			rowData := []any{}
			if sqles.IsOkResult(row) { //This is for insert statements
				okResult = true
				sqlOkResult := sqles.GetOkResult(row)
				if sqlOkResult.RowsAffected > 0 {
					matrix = append(matrix, rowData)
				} else {
					if sqlOkResult.InsertID > 0 {
						rowData = append(rowData, sqlOkResult.InsertID)
						matrix = append(matrix, rowData)
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
func QueryWithBindings(te *engine.TierceronEngine, query string, bindings map[string]sqles.Expression, queryLock *sync.Mutex) (string, []string, [][]any, error) {
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
		if strings.Contains(queryErr.Error(), "duplicate") {
			return "", nil, nil, errors.New("Duplicate primary key found.")
		}
		return "", nil, nil, queryErr
	}

	columns := []string{}
	matrix := [][]any{}
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
			rowData := []any{}
			if sqles.IsOkResult(row) { //This is for insert statements
				okResult = true
				sqlOkResult := sqles.GetOkResult(row)
				if sqlOkResult.RowsAffected > 0 {
					matrix = append(matrix, rowData)
				} else {
					if sqlOkResult.InsertID > 0 {
						rowData = append(rowData, sqlOkResult.InsertID)
						matrix = append(matrix, rowData)
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

// Query - queries configurations using standard ANSI SQL syntax.
// Able to run query with multiple flows with bindings.
// Example: select * from ServiceTechMobileAPI.configfile
func QueryWithBindingsN(te *engine.TierceronEngine, query string, bindings map[string]sqles.Expression, queryMask uint64, bitlock bitcore.BitLock) (string, []string, [][]any, error) {
	// Create a test memory database and register it to the default engine.

	//ctx := sql.NewContext(context.Background(), sql.WithIndexRegistry(sql.NewIndexRegistry()), sql.WithViewRegistry(sql.NewViewRegistry())).WithCurrentDB(te.Database.Name())
	//ctx := sql.NewContext(context.Background()).WithCurrentDB(te.Database.Name())
	ctx := sql.NewContext(context.Background())
	ctx.WithQuery(query)
	bitlock.Lock(queryMask)
	//	te.Context = ctx
	schema, r, queryErr := te.Engine.QueryWithBindings(ctx, query, bindings)
	bitlock.Unlock(queryMask)
	if queryErr != nil {
		if strings.Contains(queryErr.Error(), "duplicate") {
			return "", nil, nil, errors.New("Duplicate primary key found.")
		}
		return "", nil, nil, queryErr
	}

	columns := []string{}
	matrix := [][]any{}
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
			bitlock.Lock(queryMask)
			row, err := r.Next(ctx)
			bitlock.Unlock(queryMask)
			if err == io.EOF {
				break
			}
			rowData := []any{}
			if sqles.IsOkResult(row) { //This is for insert statements
				okResult = true
				sqlOkResult := sqles.GetOkResult(row)
				if sqlOkResult.RowsAffected > 0 {
					matrix = append(matrix, rowData)
				} else {
					if sqlOkResult.InsertID > 0 {
						rowData = append(rowData, sqlOkResult.InsertID)
						matrix = append(matrix, rowData)
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
