package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/trimble-oss/tierceron/buildopts"
	eUtils "github.com/trimble-oss/tierceron/utils"
	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"

	//mysql and mssql go libraries
	_ "github.com/denisenkom/go-mssqldb"
)

func (s *Server) authUser(config *eUtils.DriverConfig, mod *helperkv.Modifier, operatorId string, operatorPassword string) (bool, string, error) {
	connInfo, err := mod.ReadData("apiLogins/meta")
	if err != nil {
		return false, "", err
	}

	var url, username, password string
	url, ok := connInfo["sessionDB"].(string)
	if !ok {
		return false, "", fmt.Errorf("database connection not a string or not found")
	}
	username, ok = connInfo["user"].(string)
	if !ok {
		return false, "", fmt.Errorf("username connection not a string or not found")
	}
	password, ok = connInfo["pass"].(string)
	if !ok {
		return false, "", fmt.Errorf("password connection not a string or not found")
	}

	driver, server, port, dbname, parseError := parseURL(config, url)
	if parseError != nil {
		return false, "", parseError
	}

	if len(port) == 0 {
		port = "1433"
	}
	db, err := sql.Open(driver, ("server=" + server + ";user id=" + username + ";password=" + password + ";port=" + port + ";database=" + dbname + ";encrypt=true;TrustServerCertificate=true"))
	if db != nil {
		defer db.Close()
	}
	if err != nil {
		return false, "", err
	}

	return buildopts.Authorize(db, operatorId, operatorPassword)
}

func (s *Server) getActiveSessions(config *eUtils.DriverConfig, env string) ([]map[string]interface{}, error) {
	mod, err := helperkv.NewModifier(false, s.VaultToken, s.VaultAddr, "nonprod", nil, true, s.Log)
	if err != nil {
		return nil, err
	}
	mod.Env = env
	connInfo, err := mod.ReadData("apiLogins/meta")

	var url, username, password string
	url, ok := connInfo["sessionDB"].(string)
	if !ok {
		return nil, fmt.Errorf("database connection not a string or not found")
	}
	username, ok = connInfo["user"].(string)
	if !ok {
		return nil, fmt.Errorf("username connection not a string or not found")
	}
	password, ok = connInfo["pass"].(string)
	if !ok {
		return nil, fmt.Errorf("password connection not a string or not found")
	}

	driver, server, port, dbname, parseError := parseURL(config, url)
	if err != nil {
		return nil, parseError
	}
	if len(port) == 0 {
		port = "1433"
	}
	db, err := sql.Open(driver, ("server=" + server + ";user id=" + username + ";password=" + password + ";port=" + port + ";database=" + dbname + ";encrypt=true;TrustServerCertificate=true"))
	if db != nil {
		defer db.Close()
	}
	if err != nil {
		return nil, err
	}

	return coreopts.ActiveSessions(db)
}

func parseURL(config *eUtils.DriverConfig, url string) (string, string, string, string, error) {
	//only works with jdbc:mysql or jdbc:sqlserver.
	regex := regexp.MustCompile(`(?i)(mysql|sqlserver)://([\w\-\.]+)(?::(\d{0,5}))?(?:/|.*;DatabaseName=)(\w+).*`)
	m := regex.FindStringSubmatch(url)
	if m == nil {
		parseFailureErr := errors.New("incorrect URL format")
		eUtils.LogErrorMessage(config, parseFailureErr.Error(), true)
		return "", "", "", "", parseFailureErr
	}
	return m[1], m[2], m[3], m[4], nil
}

func (s *Server) getVaultSessions(env string) ([]map[string]interface{}, error) {
	mod, err := helperkv.NewModifier(false, s.VaultToken, s.VaultAddr, "nonprod", nil, true, s.Log)
	if err != nil {
		return nil, err
	}
	mod.Env = ""

	sessions := []map[string]interface{}{}
	paths, err := mod.List("apiLogins/"+env, s.Log)
	if paths == nil {
		return nil, fmt.Errorf("Nothing found under apiLogins/" + env)
	}
	if err != nil {
		return nil, err
	}
	mod.Env = env

	// Pass through all registered users
	var id int
	if users, ok := paths.Data["keys"].([]interface{}); ok {
		for _, user := range users {
			if user == "meta" {
				continue
			}
			userData, err := mod.ReadData("apiLogins/" + user.(string))
			if err != nil {
				return nil, err
			}

			issued, err := userData["Issued"].(json.Number).Int64()
			if err != nil {
				return nil, err
			}
			expires, err := userData["Expires"].(json.Number).Int64()
			if err != nil {
				return nil, err
			}
			// Check if session has expired
			if expires < time.Now().Unix() {
				userData["Issued"] = -1
				userData["Expires"] = -1
			} else {
				sessions = append(sessions, map[string]interface{}{
					"ID":        id,
					"User":      strings.TrimSpace(user.(string)),
					"LastLogIn": issued,
				})
				id++
			}
		}
	}

	return sessions, nil
}
