package harbingeropts

import (
	"crypto/md5"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	sqle "github.com/dolthub/go-mysql-server"
	"github.com/dolthub/go-mysql-server/server"
	sqles "github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/information_schema"
	flowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/opts/insecure"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	trcutil "github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"

	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
)

// Folder prefix for _seed and _templates.  This function takes a list of paths and looking
// at the first entry, retrieve an embedded folder prefix.
func GetFolderPrefix(custom []string) string {
	if len(custom) > 0 && len(custom[0]) > 0 {
		var ti, endTi int
		ti = strings.Index(custom[0], "_templates")
		endTi = 0

		for endTi = ti; endTi > 0; endTi-- {
			if custom[0][endTi] == '/' {
				endTi = endTi + 1
				break
			}
		}
		return custom[0][endTi:ti]
	}
	return "trc"
}

// GetDatabaseName - returns a name to be used by TrcDb.
func GetDatabaseName() string {
	return ""
}

func engineQuery(engine *sqle.Engine, ctx *sqles.Context, query string) (string, []string, [][]interface{}, error) {
	schema, r, queryErr := engine.Query(ctx, query)
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
			row, err := r.Next(ctx)
			if err == io.EOF {
				break
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

// Used to define a database interface for querying TrcDb.
// Builds interface for TrcDB
func BuildInterface(driverConfig *config.DriverConfig, goMod *kv.Modifier, tfmContextInterface interface{}, vaultDatabaseConfig map[string]interface{}, serverListenerInterface interface{}) error {
	serverListener := serverListenerInterface.(server.ServerEventListener)
	tfmContext := tfmContextInterface.(*flowcore.TrcFlowMachineContext)
	interfaceUrl, parseErr := url.Parse(vaultDatabaseConfig["vaddress"].(string))

	if parseErr != nil {
		eUtils.LogErrorMessage(&driverConfig.CoreConfig, "Could not parse address for interface. Failing to start interface", false)
		return parseErr
	}

	if _, ok := vaultDatabaseConfig["dbport"]; ok {
		vaultDatabaseConfig["vaddress"] = strings.Split(interfaceUrl.Host, ":")[0] + ":" + vaultDatabaseConfig["dbport"].(string)
	} else {
		eUtils.LogErrorMessage(&driverConfig.CoreConfig, "Missing port. Failing to start interface", false)
		return errors.New("Missing port for interface")
	}
	driverConfig.CoreConfig.Log.Println("Starting SQL Interface.")
	engine := sqle.NewDefault(
		sqles.NewDatabaseProvider(
			tfmContext.TierceronEngine.Database,
			information_schema.NewInformationSchemaDatabase(),
		))

	driverConfig.CoreConfig.Log.Println("Loading cert from vault.")
	pwd, _ := os.Getwd()

	//Grab certs
	driverConfig.CoreConfig.WantCerts = true
	_, certData, certLoaded, ctErr := trcutil.ConfigTemplate(driverConfig, goMod, strings.Split(pwd, "tierceron")[0]+"tierceron/trcvault/trc_templates/Common/db_cert.pem.mf.tmpl", true, "Common", "db_cert", true, true)
	if ctErr != nil || !certLoaded || len(certData) == 0 {
		if ctErr != nil {
			eUtils.LogErrorMessage(&driverConfig.CoreConfig, ctErr.Error(), false)
		}
		return errors.New("Failed to retrieve cert.")
	}

	_, keyData, keyLoaded, key_Err := trcutil.ConfigTemplate(driverConfig, goMod, strings.Split(pwd, "tierceron")[0]+"tierceron/trcvault/trc_templates/Common/db_key.pem.mf.tmpl", true, "Common", "db_key", true, true)
	if ctErr != nil || !keyLoaded || len(keyData) == 0 {
		if key_Err != nil {
			eUtils.LogErrorMessage(&driverConfig.CoreConfig, key_Err.Error(), false)
		}
		return errors.New("Failed to retrieve key.")
	}

	key_pair, key_pair_err := tls.X509KeyPair([]byte(certData[1]), []byte(keyData[1]))
	if key_pair_err != nil {
		eUtils.LogErrorMessage(&driverConfig.CoreConfig, key_pair_err.Error(), false)
	}

	certPool, _ := x509.SystemCertPool()
	if certPool == nil {
		certPool = x509.NewCertPool()
	}
	serverConfig := server.Config{
		Protocol: "tcp",
		Address:  ":" + strings.TrimSpace(vaultDatabaseConfig["dbport"].(string)),
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{key_pair},
			//ClientAuth:         tls.RequireAndVerifyClientCert,
			ServerName:         strings.TrimSpace(strings.Split(vaultDatabaseConfig["vaddress"].(string), ":")[0]),
			InsecureSkipVerify: insecure.IsInsecure(),
			MinVersion:         tls.VersionTLS12,
		},
		RequireSecureTransport: true,
	}

	if coreopts.BuildOptions.IsLocalEndpoint(vaultDatabaseConfig["vaddress"].(string)) {
		serverIP := net.ParseIP("127.0.0.1") // Change to local IP for self signed cert local debugging
		serverConfig.TLSConfig.VerifyPeerCertificate = func(certificates [][]byte, verifiedChains [][]*x509.Certificate) error {
			for _, certChain := range verifiedChains {
				for _, cert := range certChain {
					if cert.IPAddresses != nil {
						for _, ip := range cert.IPAddresses {
							if ip.Equal(serverIP) {
								return nil
							}
						}
					}
				}
			}
			return errors.New("TLS certificate verification failed (IP SAN mismatch)")
		}
	}

	dbserver, serverErr := server.NewServer(serverConfig, engine, server.DefaultSessionBuilder, serverListener)
	if serverErr != nil {
		eUtils.LogErrorMessage(&driverConfig.CoreConfig, "Failed to start server:"+serverErr.Error(), false)
		return serverErr
	}

	//Adding auth
	if okSourcePath, okDestPath := vaultDatabaseConfig["dbuser"], vaultDatabaseConfig["dbpassword"]; okSourcePath != nil && okDestPath != nil {
		//engine.Analyzer.Catalog.GrantTables.AddSuperUser(vaultDatabaseConfig["dbuser"].(string), vaultDatabaseConfig["dbpassword"].(string))
		h := md5.New()
		io.WriteString(h, strconv.FormatInt(time.Now().Unix(), 10))
		superRandom := string(h.Sum(nil))

		engine.Analyzer.Catalog.MySQLDb.AddSuperUser("", "", superRandom) //Use for permission set up -> deleted before setup finishes
		ctx := tfmContext.TierceronEngine.Context
		cidrBlockSlice := strings.Split(vaultDatabaseConfig["cidrblock"].(string), ",")
		for _, cidrBlock := range cidrBlockSlice {
			cidrBlock = strings.TrimSpace(cidrBlock)
			_, _, _, queryErr := engineQuery(engine, ctx, "CREATE USER '"+vaultDatabaseConfig["dbuser"].(string)+"'@'"+cidrBlock+"' IDENTIFIED BY '"+vaultDatabaseConfig["dbpassword"].(string)+"'")
			if queryErr != nil {
				eUtils.LogErrorMessage(&driverConfig.CoreConfig, "Failed to select user - "+queryErr.Error(), false)
			}

			_, _, tableNameMatrix, showQueryErr := engineQuery(engine, ctx, "SHOW TABLES FROM "+tfmContext.TierceronEngine.Database.Name())
			if showQueryErr != nil {
				eUtils.LogErrorMessage(&driverConfig.CoreConfig, "Failed to grant user permissions - 1 :"+showQueryErr.Error(), false)
			}

			for _, tableNameListInterface := range tableNameMatrix {
				for _, tableName := range tableNameListInterface {
					if !strings.Contains(tableName.(string), "Changes") {
						if strings.Contains(tableName.(string), "DataFlowStatistics") {
							_, _, _, queryErr = engineQuery(engine, ctx, "GRANT SELECT ON "+tfmContext.TierceronEngine.Database.Name()+"."+tableName.(string)+" TO '"+vaultDatabaseConfig["dbuser"].(string)+"'@'"+cidrBlock+"'")
						} else if strings.Contains(tableName.(string), "TierceronFlow") {
							_, _, _, queryErr = engineQuery(engine, ctx, "GRANT SELECT,INSERT,UPDATE ON "+tfmContext.TierceronEngine.Database.Name()+"."+tableName.(string)+" TO '"+vaultDatabaseConfig["dbuser"].(string)+"'@'"+cidrBlock+"'")
						} else {
							_, _, _, queryErr = engineQuery(engine, ctx, "GRANT SELECT,INSERT,UPDATE,DELETE ON "+tfmContext.TierceronEngine.Database.Name()+"."+tableName.(string)+" TO '"+vaultDatabaseConfig["dbuser"].(string)+"'@'"+cidrBlock+"'")
						}
						if queryErr != nil {
							eUtils.LogErrorMessage(&driverConfig.CoreConfig, "Failed to grant user permissions - 2a :"+queryErr.Error(), false)
						}
					} else {
						_, _, _, queryErr = engineQuery(engine, ctx, "GRANT INSERT,UPDATE,DELETE ON "+tfmContext.TierceronEngine.Database.Name()+"."+tableName.(string)+" TO '"+vaultDatabaseConfig["dbuser"].(string)+"'@'"+cidrBlock+"'")
						if queryErr != nil {
							eUtils.LogErrorMessage(&driverConfig.CoreConfig, "Failed to grant user permissions - 2b :"+queryErr.Error(), false)
						}
					}
				}
			}

			_, _, _, queryErr = engineQuery(engine, ctx, "GRANT SELECT ON INFORMATION_SCHEMA.* TO '"+vaultDatabaseConfig["dbuser"].(string)+"'@'"+cidrBlock+"'")
			if queryErr != nil {
				eUtils.LogErrorMessage(&driverConfig.CoreConfig, "Failed to grant user permissions - 3 :"+queryErr.Error(), false)
			}

			_, _, _, queryErr = engineQuery(engine, ctx, "REVOKE INSERT,UPDATE,DELETE ON "+tfmContext.TierceronEngine.Database.Name()+"."+"DataFlowStatistics FROM '"+vaultDatabaseConfig["dbuser"].(string)+"'@'"+cidrBlock+"'")
			if queryErr != nil {
				eUtils.LogErrorMessage(&driverConfig.CoreConfig, "Failed to grant user permissions - 4 :"+queryErr.Error(), false)
			}
		}
		_, _, _, queryErr := engineQuery(engine, ctx, "FLUSH PRIVILEGES")
		if queryErr != nil {
			eUtils.LogErrorMessage(&driverConfig.CoreConfig, "Failed refresh permissions for users- "+queryErr.Error(), false)
			dbserver = nil
			goto permsfailure
		}

		_, _, _, queryErr = engineQuery(engine, ctx, "DELETE USER FROM Mysql.user where USER=''")
		if queryErr != nil {
			eUtils.LogErrorMessage(&driverConfig.CoreConfig, "Failed to delete user used to set up permissions:"+queryErr.Error(), false)
			dbserver = nil
			goto permsfailure
		}

		eUtils.LogErrorMessage(&driverConfig.CoreConfig, "Permissions have been set up.", false)
	}

	go func(tfC *flowcore.TrcFlowMachineContext, vdc map[string]interface{}) {
		for {
			select {
			case <-tfmContext.PermissionChan:
				// TODO: Do something if you feel like it...
				break
			}
		}
	}(tfmContext, vaultDatabaseConfig)

	go dbserver.Start()

	driverConfig.CoreConfig.Log.Println("SQL Interface Started.")
permsfailure:
	return nil
}
