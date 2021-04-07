package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"Vault.Whoville/vaulthelper/kv"
	configcore "VaultConfig.Bootstrap/configcore"

	//mysql and mssql go libraries
	_ "github.com/denisenkom/go-mssqldb"
)

func (s *Server) authUser(mod *kv.Modifier, operatorId string, operatorPassword string) (bool, string, error) {
	connInfo, err := mod.ReadData("apiLogins/meta")

	var url, username, password string
	url, ok := connInfo["sessionDB"].(string)
	if !ok {
		return false, "", fmt.Errorf("Database connection not a string or not found")
	}
	username, ok = connInfo["user"].(string)
	if !ok {
		return false, "", fmt.Errorf("Username connection not a string or not found")
	}
	password, ok = connInfo["pass"].(string)
	if !ok {
		return false, "", fmt.Errorf("Password connection not a string or not found")
	}

	driver, server, port, dbname := parseURL(url)
	if len(port) == 0 {
		port = "1433"
	}
	db, err := sql.Open(driver, ("server=" + server + ";user id=" + username + ";password=" + password + ";port=" + port + ";database=" + dbname + ";encrypt=true;TrustServerCertificate=true"))
	defer db.Close()
	if err != nil {
		return false, "", err
	}

	return configcore.Authorize(db, operatorId, operatorPassword)
}

func (s *Server) getActiveSessions(env string) ([]configcore.Session, error) {
	mod, err := kv.NewModifier(s.VaultToken, s.VaultAddr, "nonprod", nil)
	if err != nil {
		return nil, err
	}
	mod.Env = env
	connInfo, err := mod.ReadData("apiLogins/meta")

	var url, username, password string
	url, ok := connInfo["sessionDB"].(string)
	if !ok {
		return nil, fmt.Errorf("Database connection not a string or not found")
	}
	username, ok = connInfo["user"].(string)
	if !ok {
		return nil, fmt.Errorf("Username connection not a string or not found")
	}
	password, ok = connInfo["pass"].(string)
	if !ok {
		return nil, fmt.Errorf("Password connection not a string or not found")
	}

	driver, server, port, dbname := parseURL(url)
	if len(port) == 0 {
		port = "1433"
	}
	db, err := sql.Open(driver, ("server=" + server + ";user id=" + username + ";password=" + password + ";port=" + port + ";database=" + dbname + ";encrypt=true;TrustServerCertificate=true"))
	defer db.Close()
	if err != nil {
		return nil, err
	}

	return configcore.ActiveSessions(db)
}

func parseURL(url string) (string, string, string, string) {
	//only works with jdbc:mysql or jdbc:sqlserver.
	regex := regexp.MustCompile(`(?i)(mysql|sqlserver)://([\w\-\.]+)(?::(\d{0,5}))?(?:/|.*;DatabaseName=)(\w+).*`)
	m := regex.FindStringSubmatch(url)
	if m == nil {
		panic(errors.New("incorrect URL format"))
	}
	return m[1], m[2], m[3], m[4]
}

func (s *Server) getVaultSessions(env string) ([]configcore.Session, error) {
	mod, err := kv.NewModifier(s.VaultToken, s.VaultAddr, "nonprod", nil)
	if err != nil {
		return nil, err
	}
	mod.Env = ""

	sessions := []configcore.Session{}
	paths, err := mod.List("apiLogins/" + env)
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
				sessions = append(sessions, configcore.Session{
					ID:        id,
					User:      strings.TrimSpace(user.(string)),
					LastLogIn: issued,
				})
				id++
			}
		}
	}

	return sessions, nil
}
