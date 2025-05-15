package harbingeropts

import (
	"crypto/md5"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	sqle "github.com/dolthub/go-mysql-server"
	"github.com/dolthub/go-mysql-server/server"
	"github.com/dolthub/go-mysql-server/sql"
	sqles "github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/information_schema"
	"github.com/dolthub/go-mysql-server/sql/mysql_db"
	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	trcflowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/opts/insecure"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	trcutil "github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase/utils"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
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

func BuildTableGrant(tableName string) (string, error) {
	switch tableName {
	case "DataFlowStatistics":
		//	return "GRANT SELECT ON %s.%s TO '%s'@'%s'", nil
	}
	return "", errors.New("use default grant")
}

func TableGrantNotify(tfmContext flowcore.FlowMachineContext, tableName string) {
	// TODO: Add notification that table grant has been provided.
	if tableName == "DataFlowStatistics" {
		// if registerEnterpriseFlowContext, refOk := tfmContext.FlowMap[flowcore.FlowNameType("EnterpriseRegistrations")]; refOk {
		// 	go func(refContext flowcore.FlowContext) {
		// 		refContext.ContextNotifyChan <- true
		// 	}(registerEnterpriseFlowContext)
		// }
	}
}

// Used to define a database interface for querying TrcDb.
// Builds interface for TrcDB
func BuildInterface(driverConfig *config.DriverConfig, goMod *kv.Modifier, tfmContextInterface interface{}, vaultDatabaseConfig map[string]interface{}, serverListenerInterface interface{}) error {
	serverListener := serverListenerInterface.(server.ServerEventListener)
	tfmContext := tfmContextInterface.(*trcflowcore.TrcFlowMachineContext)
	interfaceUrl, parseErr := url.Parse(vaultDatabaseConfig["vaddress"].(string))

	if parseErr != nil {
		eUtils.LogErrorMessage(driverConfig.CoreConfig, "Could not parse address for interface. Failing to start interface", false)
		return parseErr
	}

	if _, ok := vaultDatabaseConfig["dbport"]; ok {
		vaultDatabaseConfig["vaddress"] = strings.Split(interfaceUrl.Host, ":")[0] + ":" + vaultDatabaseConfig["dbport"].(string)
	} else {
		eUtils.LogErrorMessage(driverConfig.CoreConfig, "Missing port. Failing to start interface", false)
		return errors.New("Missing port for interface")
	}
	eUtils.LogInfo(driverConfig.CoreConfig, "Starting SQL Interface.")
	engine := sqle.NewDefault(
		sqles.NewDatabaseProvider(
			tfmContext.TierceronEngine.Database,
			information_schema.NewInformationSchemaDatabase(),
		))
	engine.Analyzer.Catalog.MySQLDb.SetPersister(&mysql_db.NoopPersister{})

	eUtils.LogInfo(driverConfig.CoreConfig, "Loading cert from vault.")
	pwd, _ := os.Getwd()

	//Grab certs
	driverConfig.CoreConfig.WantCerts = true
	_, certData, certLoaded, ctErr := trcutil.ConfigTemplate(driverConfig, goMod, strings.Split(pwd, "tierceron")[0]+"tierceron/trcvault/trc_templates/Common/db_cert.pem.mf.tmpl", true, "Common", "db_cert", true, true)
	if ctErr != nil || !certLoaded || len(certData) == 0 {
		if ctErr != nil {
			eUtils.LogErrorMessage(driverConfig.CoreConfig, ctErr.Error(), false)
		}
		return errors.New("Failed to retrieve cert.")
	}

	_, keyData, keyLoaded, key_Err := trcutil.ConfigTemplate(driverConfig, goMod, strings.Split(pwd, "tierceron")[0]+"tierceron/trcvault/trc_templates/Common/db_key.pem.mf.tmpl", true, "Common", "db_key", true, true)
	if ctErr != nil || !keyLoaded || len(keyData) == 0 {
		if key_Err != nil {
			eUtils.LogErrorMessage(driverConfig.CoreConfig, key_Err.Error(), false)
		}
		return errors.New("Failed to retrieve key.")
	}

	key_pair, key_pair_err := tls.X509KeyPair([]byte(certData[1]), []byte(keyData[1]))
	if key_pair_err != nil {
		eUtils.LogErrorMessage(driverConfig.CoreConfig, key_pair_err.Error(), false)
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
		eUtils.LogErrorMessage(driverConfig.CoreConfig, "Failed to start server:"+serverErr.Error(), false)
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

		if _, ok := vaultDatabaseConfig["controller"].(bool); !ok {
			_, _, _, placeHolderErr := engineQuery(engine, ctx, "CREATE TABLE "+tfmContext.TierceronEngine.Database.Name()+".placeholder (placeholder int);")
			if placeHolderErr != nil {
				eUtils.LogErrorMessage(driverConfig.CoreConfig, "Failed to create placeholder table to keep connection alive - "+placeHolderErr.Error(), false)
			}
		}

		cidrBlockSlice := strings.Split(vaultDatabaseConfig["cidrblock"].(string), ",")
		for _, cidrBlock := range cidrBlockSlice {
			dfsUserCreated := ""
			cidrBlock = strings.TrimSpace(cidrBlock)
			_, _, _, queryErr := engineQuery(engine, ctx, "CREATE USER '"+vaultDatabaseConfig["dbuser"].(string)+"'@'"+cidrBlock+"' IDENTIFIED BY '"+vaultDatabaseConfig["dbpassword"].(string)+"'")

			if queryErr != nil {
				eUtils.LogErrorMessage(driverConfig.CoreConfig, fmt.Sprintf("Failed to create user for cidr - %s", cidrBlock), false)
			}

			_, _, _, queryErr = engineQuery(engine, ctx, "GRANT SELECT ON INFORMATION_SCHEMA.* TO '"+vaultDatabaseConfig["dbuser"].(string)+"'@'"+cidrBlock+"'")
			if queryErr != nil {
				eUtils.LogErrorMessage(driverConfig.CoreConfig, fmt.Sprintf("Failed to grant user permissions - 3 - %s", cidrBlock), false)
			}

			if _, ok := vaultDatabaseConfig["controller"].(bool); !ok {
				_, _, _, queryErr = engineQuery(engine, ctx, "GRANT SELECT ON "+tfmContext.TierceronEngine.Database.Name()+".placeholder TO '"+vaultDatabaseConfig["dbuser"].(string)+"'@'"+cidrBlock+"'")
				if queryErr != nil {
					eUtils.LogErrorMessage(driverConfig.CoreConfig, fmt.Sprintf("Failed to grant user permissions - 4 - %s", cidrBlock), false)
				}
			}

			if len(dfsUserCreated) > 0 {
				_, _, _, queryErr = engineQuery(engine, ctx, "GRANT SELECT ON INFORMATION_SCHEMA.* TO '"+dfsUserCreated+"'@'"+cidrBlock+"'")
				if queryErr != nil {
					eUtils.LogErrorMessage(driverConfig.CoreConfig, fmt.Sprintf("Failed to grant user permissions - 5 - %s", cidrBlock), false)
				}

				if _, ok := vaultDatabaseConfig["controller"].(bool); !ok {
					_, _, _, queryErr = engineQuery(engine, ctx, "GRANT SELECT ON placeholder TO '"+vaultDatabaseConfig["dbuser"].(string)+"'@'"+cidrBlock+"'")
					if queryErr != nil {
						eUtils.LogErrorMessage(driverConfig.CoreConfig, fmt.Sprintf("Failed to grant user permissions - 6 - %s", cidrBlock), false)
					}
				}
			}

			_, _, _, queryErr = engineQuery(engine, ctx, "REVOKE INSERT,UPDATE,DELETE ON "+tfmContext.TierceronEngine.Database.Name()+"."+"DataFlowStatistics FROM '"+vaultDatabaseConfig["dbuser"].(string)+"'@'"+cidrBlock+"'")
			if queryErr != nil {
				eUtils.LogErrorMessage(driverConfig.CoreConfig, fmt.Sprintf("Failed to grant user permissions - 7 - %s", cidrBlock), false)
			}

			_, _, _, queryErr = engineQuery(engine, ctx, "REVOKE INSERT,UPDATE,DELETE ON "+tfmContext.TierceronEngine.Database.Name()+"."+"placeholder FROM '"+vaultDatabaseConfig["dbuser"].(string)+"'@'"+cidrBlock+"'")
			if queryErr != nil {
				eUtils.LogErrorMessage(driverConfig.CoreConfig, fmt.Sprintf("Failed to grant user permissions - 8 - %s", cidrBlock), false)
			}
			ctx.Done()
			tfmContext.TierceronEngine.Context = sql.NewEmptyContext()
		}
		_, _, _, queryErr := engineQuery(engine, ctx, "FLUSH PRIVILEGES")
		if queryErr != nil {
			eUtils.LogErrorMessage(driverConfig.CoreConfig, "Failed refresh permissions for users", false)
			dbserver = nil
			goto permsfailure
		}

		_, _, _, queryErr = engineQuery(engine, ctx, "DELETE USER FROM Mysql.user where USER=''")
		if queryErr != nil {
			eUtils.LogErrorMessage(driverConfig.CoreConfig, "Failed to delete user used to set up permissions.", false)
			dbserver = nil
			goto permsfailure
		}

		eUtils.LogInfo(driverConfig.CoreConfig, "Permissions have been set up.")

		//Set off listeners for when tables are ready.
		go func(tfC *trcflowcore.TrcFlowMachineContext, vdc map[string]interface{}) {
			for {
				select {
				case permUpdate := <-tfmContext.PermissionChan:
					tableName := permUpdate.TableName
					permission := false
					if permUpdate.CurrentState == 2 {
						permission = true
					}

					var queryErr error
					h := md5.New()
					randomTime := rand.Int63n(time.Now().Unix()-int64(rand.Intn(99999999))) + int64(rand.Intn(99999999))
					randomNow := time.Unix(randomTime, 0)
					io.WriteString(h, randomNow.String())
					superRandom := string(h.Sum(nil))

					engine.Analyzer.Catalog.MySQLDb.AddSuperUser("", "", superRandom) //Use for permission set up -> deleted before setup finishes
					cidrBlockSlice := strings.Split(vdc["cidrblock"].(string), ",")
					tables := []string{tableName, tableName + "_Changes"}

					if permission {
						for _, tableN := range tables {
							for _, cidrBlock := range cidrBlockSlice {
								if !strings.HasSuffix(tableN, "Changes") {
									if strings.HasPrefix(tableN, "DataFlowStatistics") {
										_, _, _, queryErr = engineQuery(engine, ctx, "GRANT SELECT ON "+tfC.TierceronEngine.Database.Name()+"."+tableN+" TO '"+vdc["dbuser"].(string)+"'@'"+cidrBlock+"'")

										//If DFS user configs are loaded, create that user.
										if dfsUser, ok := vaultDatabaseConfig["dfsUser"].(string); ok {
											if dfsPass, ok := vaultDatabaseConfig["dfsPass"].(string); ok {
												dfsUserCreated := dfsUser
												_, _, _, queryErr := engineQuery(engine, ctx, "CREATE USER '"+dfsUser+"'@'"+cidrBlock+"' IDENTIFIED BY '"+dfsPass+"'")
												if len(dfsUserCreated) > 0 {
													_, _, _, queryErr = engineQuery(engine, ctx, "GRANT SELECT ON "+tfC.TierceronEngine.Database.Name()+"."+tableN+" TO '"+dfsUserCreated+"'@'"+cidrBlock+"'")
												}

												if queryErr != nil {
													eUtils.LogErrorMessage(driverConfig.CoreConfig, fmt.Sprintf("Failed to create dfs interface user - %s", cidrBlock), false)
												}
											}
										}

									} else if strings.Contains(tableN, "TierceronFlow") {
										_, _, _, queryErr = engineQuery(engine, ctx, "GRANT SELECT,INSERT,UPDATE ON "+tfC.TierceronEngine.Database.Name()+"."+tableN+" TO '"+vdc["dbuser"].(string)+"'@'"+cidrBlock+"'")
									} else {
										// Override the default grant when provided.
										if grant, err := BuildOptions.BuildTableGrant(tableN); err == nil {
											if strings.Contains(grant, "GRANT") && strings.Count(grant, "%s") == 4 {
												_, _, _, queryErr = engineQuery(engine, ctx, fmt.Sprintf(grant, tfC.TierceronEngine.Database.Name(), tableN, vdc["dbuser"].(string), cidrBlock))
											} else if strings.Contains(grant, "GRANT") && strings.Count(grant, "%s") == 3 {
												_, _, _, queryErr = engineQuery(engine, ctx, fmt.Sprintf(grant, tfC.TierceronEngine.Database.Name(), tableN, vdc["dbuser"].(string)))
											} else {
												queryErr = errors.New("unexpected grant format")
											}
										} else {
											if err.Error() == "use default grant" {
												_, _, _, queryErr = engineQuery(engine, ctx, "GRANT SELECT,INSERT,UPDATE,DELETE ON "+tfC.TierceronEngine.Database.Name()+"."+tableN+" TO '"+vdc["dbuser"].(string)+"'@'"+cidrBlock+"'")
											} else {
												queryErr = err
											}
										}
									}
								} else {
									_, _, _, queryErr = engineQuery(engine, ctx, "GRANT TRIGGER,INSERT,UPDATE,DELETE ON "+tfC.TierceronEngine.Database.Name()+"."+tableN+" TO '"+vdc["dbuser"].(string)+"'@'"+cidrBlock+"'")
									if queryErr != nil {
										eUtils.LogErrorMessage(driverConfig.CoreConfig, fmt.Sprintf("Failed to grant user permissions - 2b for %s", tableN), false)
									}
								}

								if queryErr != nil {
									eUtils.LogErrorMessage(driverConfig.CoreConfig, "Failed to grant user permissions - 2a for"+tableN+":"+queryErr.Error(), false)
								} else {
									// Notify that the table grant has been provided.
									if tableName != "" {
										BuildOptions.TableGrantNotify(tfmContext, tableN)
									}
								}

							}
						}
						_, _, _, queryErr = engineQuery(engine, ctx, "FLUSH PRIVILEGES")
						if queryErr != nil {
							eUtils.LogErrorMessage(driverConfig.CoreConfig, fmt.Sprintf("Failed refresh permissions for users for %s", tableName), false)
						}

						_, _, _, queryErr = engineQuery(engine, ctx, "DELETE USER FROM Mysql.user where USER=''")
						if queryErr != nil {
							eUtils.LogErrorMessage(driverConfig.CoreConfig, "Failed to delete user used to set up permissions for"+tableName+":"+queryErr.Error(), false)
						}
						eUtils.LogInfo(driverConfig.CoreConfig, "Permissions have been enabled for "+tableName+".")
					} else {
						//disable permissions
						for _, tableN := range tables {
							for _, cidrBlock := range cidrBlockSlice {
								if !strings.HasSuffix(tableN, "Changes") {
									if strings.HasPrefix(tableN, "DataFlowStatistics") {
										if dfsUser, ok := vaultDatabaseConfig["dfsUser"].(string); ok {
											if len(dfsUser) > 0 {
												_, _, _, queryErr = engineQuery(engine, ctx, "REVOKE ALL ON "+tfC.TierceronEngine.Database.Name()+"."+tableN+" FROM '"+dfsUser+"'@'"+cidrBlock+"'")
											}

											if queryErr != nil {
												eUtils.LogErrorMessage(driverConfig.CoreConfig, fmt.Sprintf("Failed to create dfs interface user -  %s", tableName), false)
											}
										}
									}
								}

								_, _, _, queryErr = engineQuery(engine, ctx, "REVOKE ALL ON "+tfC.TierceronEngine.Database.Name()+"."+tableN+" FROM '"+vdc["dbuser"].(string)+"'@'"+cidrBlock+"'")
								if queryErr != nil {
									eUtils.LogErrorMessage(driverConfig.CoreConfig, "Failed refresh permissions for users for"+tableName, false)
								}
							}
						}
						_, _, _, queryErr = engineQuery(engine, ctx, "FLUSH PRIVILEGES")
						if queryErr != nil {
							eUtils.LogErrorMessage(driverConfig.CoreConfig, "Failed refresh permissions for users for"+tableName+":"+queryErr.Error(), false)
						}

						_, _, _, queryErr = engineQuery(engine, ctx, "DELETE USER FROM Mysql.user where USER=''")
						if queryErr != nil {
							eUtils.LogErrorMessage(driverConfig.CoreConfig, "Failed to delete user used to set up permissions for"+tableName+":"+queryErr.Error(), false)
						}
						eUtils.LogInfo(driverConfig.CoreConfig, "Permissions have been disabled for "+tableName+".")
					}
				}
			}
		}(tfmContext, vaultDatabaseConfig)
	}

	go dbserver.Start()

	driverConfig.CoreConfig.Log.Println("SQL Interface Started.")
permsfailure:
	return nil
}
