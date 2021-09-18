package db

import (
	"context"
	"fmt"
	"io"
	"strings"

	eUtils "Vault.Whoville/utils"
	vcutils "Vault.Whoville/vaultconfig/utils"
	"Vault.Whoville/vaulthelper/kv"
	"Vault.Whoville/vaultx/extract"

	sqle "github.com/dolthub/go-mysql-server"
	"github.com/dolthub/go-mysql-server/memory"
	"github.com/dolthub/go-mysql-server/sql"
)

// CreateEngine - creates a Tierceron query engine for query of configurations.
func CreateEngine(config eUtils.DriverConfig,
	templatePaths []string,
	env string) *TierceronEngine {
	db := memory.NewDatabase(env)
	noVault := false
	var goMod *kv.Modifier

	if config.Token != "" {
		var err error
		goMod, err = kv.NewModifier(config.Insecure, config.Token, config.VaultAddress, config.Env, config.Regions)
		if err != nil {
			panic(err)
		}
		goMod.Env = config.Env
		if config.Token == "novault" {
			noVault = true
		}
	}

	// TODO: Make this async for performance...
	for _, templatePath := range templatePaths {

		var templateResult extract.TemplateResultData
		templateResult.ValueSection = map[string]map[string]map[string]string{}
		templateResult.ValueSection["values"] = map[string]map[string]string{}

		templateResult.SecretSection = map[string]map[string]map[string]string{}
		templateResult.SecretSection["super-secrets"] = map[string]map[string]string{}

		project := ""
		service := ""

		//check for template_files directory here
		s := strings.Split(templatePath, "/")
		//figure out which path is vault_templates
		dirIndex := -1
		for j, piece := range s {
			if piece == "vault_templates" {
				dirIndex = j
			}
		}
		if dirIndex != -1 {
			project = s[dirIndex+1]
			service = s[dirIndex+2]
		}

		// Clean up service naming (Everything after '.' removed)
		dotIndex := strings.Index(service, ".")
		if dotIndex > 0 && dotIndex <= len(service) {
			service = service[0:dotIndex]
		}

		var cds *vcutils.ConfigDataStore
		if goMod != nil && !noVault {
			cds = new(vcutils.ConfigDataStore)
			cds.Init(goMod, config.SecretMode, true, project, service)
		}

		_, _, _, templateResult.TemplateDepth = extract.ToSeed(goMod,
			cds,
			templatePath,
			config.Log,
			project,
			service,
			noVault,
			&(templateResult.InterfaceTemplateSection),
			&(templateResult.ValueSection),
			&(templateResult.SecretSection),
		)

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
