package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/trimble-oss/tierceron/pkg/utils/config"

	bitcore "github.com/trimble-oss/tierceron-core/v2/bitlock"
	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	"github.com/trimble-oss/tierceron/atrium/buildopts/flowcoreopts"
	"github.com/trimble-oss/tierceron/atrium/trcdb/engine"
	coreopts "github.com/trimble-oss/tierceron/buildopts/coreopts"
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

// parseCreateTableStatement extracts table name, database name, and column definitions from a CREATE TABLE statement
// Returns (databaseName, tableName, columns, error)
// The databaseName is extracted from qualified names like "db.table" or empty string if unqualified
func parseCreateTableStatement(query string) (string, string, []ColumnDef, error) {
	// Regex to match: CREATE TABLE [IF NOT EXISTS] [db.]tablename (column definitions)
	// Capture groups: (1) optional database name, (2) table name, (3) column definitions
	createTableRegex := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:([\w]+)\.)?(\w+)\s*\((.*)\)`)
	matches := createTableRegex.FindStringSubmatch(query)
	if len(matches) < 4 {
		return "", "", nil, errors.New("invalid CREATE TABLE syntax")
	}

	databaseName := matches[1] // Empty string if not qualified
	tableName := matches[2]
	columnDefs := matches[3]

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
				return "", "", nil, fmt.Errorf("column %s has type %s; only string types (VARCHAR, CHAR, TEXT, etc.) are allowed", colName, colType)
			}

			columns = append(columns, ColumnDef{
				Name: colName,
				Type: colType,
			})
		}
	}

	if len(columns) == 0 {
		return "", "", nil, errors.New("no valid columns found in CREATE TABLE statement")
	}

	return databaseName, tableName, columns, nil
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

// isControllerDatabase checks if the given database name is the controller database
// The controller database is always "TierceronFlow" as defined in tierceron-core
func isControllerDatabase(dbName string) bool {
	return dbName == flowcore.TierceronControllerFlow.FlowName()
}

// registerFlowInTierceronFlow inserts a new flow definition into the TierceronFlow table
// and initializes it for flow processing using the generic flow pattern (process_generic flow).
// The newly created table becomes a flow that can be monitored and processed by the generic flow engine.
//
// Flow Lifecycle:
//   - On database reload: process_generic flow reads table schema from vault (templates/settings/{tableName}/schema)
//     and populates the in-memory database with table structure and any existing data
//   - On row insertion/update: Changes are captured and serialized back to vault for persistence
//   - Flow registration: Uses state=1 to mark flow as active/online for immediate discovery by process_generic
//
// Uses INSERT IGNORE to gracefully handle duplicate flow names during creation or restart scenarios.
func registerFlowInTierceronFlow(tfmContext flowcore.FlowMachineContext, tableName string) {
	if tfmContext == nil || tableName == "" {
		return
	}

	// Get the flow context for the controller database
	controllerFlowName := flowcore.TierceronControllerFlow.FlowName()
	tfContext := tfmContext.GetFlowContext(flowcore.FlowNameType(controllerFlowName))

	// Build INSERT IGNORE query to register the new flow in TierceronFlow table
	// state=1 marks flow as online/active for immediate processing by process_generic flow
	// syncMode='nosync' prevents initial sync, syncFilter empty, flowAlias=tableName for identification
	insertQuery := fmt.Sprintf(
		"INSERT IGNORE INTO %s.%s (flowName, state, syncMode, syncFilter, flowAlias, lastModified) VALUES ('%s', 1, 'nosync', '', '%s', NOW())",
		controllerFlowName,
		flowcore.TierceronControllerFlow.TableName(),
		tableName,
		tableName,
	)

	queryMap := map[string]any{"TrcQuery": insertQuery}
	flowNames := []flowcore.FlowNameType{flowcore.FlowNameType(controllerFlowName)}

	// Insert the flow definition into TierceronFlow table
	// This registration makes the flow discoverable by process_generic flow runner
	result, success := tfmContext.CallDBQuery(tfContext, queryMap, nil, false, "INSERT", flowNames, "")

	if !success || len(result) == 0 {
		tfmContext.Log(fmt.Sprintf("Failed to insert flow definition for %s into TierceronFlow\n", tableName), nil)
		return
	}

	tfmContext.Log(fmt.Sprintf("Successfully registered new flow '%s' in TierceronFlow table with state=1 (online) for process_generic\n", tableName), nil)
}

// HandleCreateTableTemplate is the public entry point for CREATE TABLE template generation.
// This is called from Query methods after intercepting a CREATE TABLE statement.
// It validates that the table is being created in a non-controller database (via fully-qualified name),
// generates a template in Vault, and registers the flow in the controller database.
//
// Usage: Client connects to controller database but specifies target database in fully-qualified name:
//
//	CREATE TABLE otherdb.tablename (columns...)
func HandleCreateTableTemplate(te *engine.TierceronEngine, query string, tfmContext flowcore.FlowMachineContext) error {
	if te == nil || te.Config.CoreConfig == nil {
		return errors.New("engine or config is nil")
	}

	// Parse the CREATE TABLE statement to extract database name, table name, and columns
	targetDB, tableName, columns, err := parseCreateTableStatement(query)
	if err != nil {
		te.Config.CoreConfig.Log.Printf("Failed to parse CREATE TABLE statement: %v\n", err)
		return err
	}

	// Validate: either a database qualifier must be specified and NOT be the controller,
	// or if unqualified, reject it to prevent accidental creation in controller database
	// Get the configured target database name (the non-controller database where tables can be created)
	allowedTargetDB := coreopts.BuildOptions.GetDatabaseName(flowcore.TrcDb)

	// Validate: database qualifier must be specified and must match the configured target database
	if targetDB == "" {
		// Unqualified table name - would default to current database (controller)
		return fmt.Errorf("CREATE TABLE requires fully-qualified name (e.g., %s.tablename) to specify target database; cannot create in controller database", allowedTargetDB)
	}
	if isControllerDatabase(targetDB) {
		// Explicitly trying to create in controller database
		return fmt.Errorf("CREATE TABLE not allowed in controller database %s; must specify target database %s", targetDB, allowedTargetDB)
	}
	if targetDB != allowedTargetDB {
		// Trying to create in a database other than the configured target
		return fmt.Errorf("CREATE TABLE only allowed in database %s, not %s", allowedTargetDB, targetDB)
	}

	// Build schema template
	schemaBytes, err := buildSchemaTemplate(tableName, columns)
	if err != nil {
		te.Config.CoreConfig.Log.Printf("Failed to build schema template: %v\n", err)
		return err
	}

	// Get a Vault modifier to push the schema to Vault
	mod, err := helperkv.NewModifierFromCoreConfig(te.Config.CoreConfig,
		"config_token_"+te.Config.CoreConfig.Env,
		te.Config.CoreConfig.Env,
		false)
	if err != nil {
		te.Config.CoreConfig.Log.Printf("Failed to create vault modifier: %v\n", err)
		return err
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
		return err
	}

	te.Config.CoreConfig.Log.Printf("Successfully pushed schema for table %s to vault at %s\n", tableName, templatePath)

	// Register the new flow in TierceronFlow table (in the controller database)
	// This creates a flow definition that can be started and managed by process_generic
	if tfmContext != nil {
		registerFlowInTierceronFlow(tfmContext, tableName)
	}

	return nil
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
	// Intercept CREATE TABLE statements before they reach the MySQL engine
	// CREATE TABLE is proxied: client connects to controller database but creates tables in other databases
	// using fully-qualified names like: CREATE TABLE otherdb.tablename (columns...)
	if flowcoreopts.BuildOptions.IsCreateTableEnabled() && isCreateTableStatement(query) {
		// Handle CREATE TABLE: generate vault template and register as generic flow
		handleErr := HandleCreateTableTemplate(te, query, te.TfmContext)
		if handleErr != nil {
			return "", nil, nil, handleErr
		}
		// Execute the CREATE TABLE in the MySQL engine so the table exists in memory
		ctx := sqles.NewContext(context.Background())
		ctx.WithQuery(query)
		queryLock.Lock()
		_, r, err := te.Engine.Query(ctx, query)
		queryLock.Unlock()
		if err != nil {
			return "", nil, nil, fmt.Errorf("failed to create table in database: %w", err)
		}
		// Consume result set to complete the operation
		for {
			queryLock.Lock()
			_, err := r.Next(ctx)
			queryLock.Unlock()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", nil, nil, err
			}
		}
		return "ok", nil, nil, nil
	}

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
	// Intercept CREATE TABLE statements before they reach the MySQL engine
	// CREATE TABLE is proxied: client connects to controller database but creates tables in other databases
	// using fully-qualified names like: CREATE TABLE otherdb.tablename (columns...)
	if flowcoreopts.BuildOptions.IsCreateTableEnabled() && isCreateTableStatement(query) {
		// Handle CREATE TABLE: generate vault template and register as generic flow
		handleErr := HandleCreateTableTemplate(te, query, te.TfmContext)
		if handleErr != nil {
			return "", nil, nil, handleErr
		}
		// Execute the CREATE TABLE in the MySQL engine so the table exists in memory
		ctx := sqles.NewContext(context.Background())
		ctx.WithQuery(query)
		bitlock.Lock(queryMask)
		_, r, err := te.Engine.Query(ctx, query)
		bitlock.Unlock(queryMask)
		if err != nil {
			return "", nil, nil, fmt.Errorf("failed to create table in database: %w", err)
		}
		// Consume result set to complete the operation
		for {
			bitlock.Lock(queryMask)
			_, err := r.Next(ctx)
			bitlock.Unlock(queryMask)
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", nil, nil, err
			}
		}
		return "ok", nil, nil, nil
	}

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
	// Intercept CREATE TABLE statements before they reach the MySQL engine
	// CREATE TABLE is proxied: client connects to controller database but creates tables in other databases
	// using fully-qualified names like: CREATE TABLE otherdb.tablename (columns...)
	if flowcoreopts.BuildOptions.IsCreateTableEnabled() && isCreateTableStatement(query) {
		// Handle CREATE TABLE: generate vault template and register as generic flow
		handleErr := HandleCreateTableTemplate(te, query, te.TfmContext)
		if handleErr != nil {
			return "", nil, nil, handleErr
		}
		// Execute the CREATE TABLE in the MySQL engine so the table exists in memory
		ctx := sql.NewContext(context.Background())
		ctx.WithQuery(query)
		queryLock.Lock()
		_, r, err := te.Engine.Query(ctx, query)
		queryLock.Unlock()
		if err != nil {
			return "", nil, nil, fmt.Errorf("failed to create table in database: %w", err)
		}
		// Consume result set to complete the operation
		for {
			queryLock.Lock()
			_, err := r.Next(ctx)
			queryLock.Unlock()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", nil, nil, err
			}
		}
		return "ok", nil, nil, nil
	}

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
	// Intercept CREATE TABLE statements before they reach the MySQL engine
	// CREATE TABLE is proxied: client connects to controller database but creates tables in other databases
	// using fully-qualified names like: CREATE TABLE otherdb.tablename (columns...)
	if flowcoreopts.BuildOptions.IsCreateTableEnabled() && isCreateTableStatement(query) {
		// Handle CREATE TABLE: generate vault template and register as generic flow
		handleErr := HandleCreateTableTemplate(te, query, te.TfmContext)
		if handleErr != nil {
			return "", nil, nil, handleErr
		}
		// Execute the CREATE TABLE in the MySQL engine so the table exists in memory
		ctx := sql.NewContext(context.Background())
		ctx.WithQuery(query)
		bitlock.Lock(queryMask)
		_, r, err := te.Engine.Query(ctx, query)
		bitlock.Unlock(queryMask)
		if err != nil {
			return "", nil, nil, fmt.Errorf("failed to create table in database: %w", err)
		}
		// Consume result set to complete the operation
		for {
			bitlock.Lock(queryMask)
			_, err := r.Next(ctx)
			bitlock.Unlock(queryMask)
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", nil, nil, err
			}
		}
		return "ok", nil, nil, nil
	}

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
