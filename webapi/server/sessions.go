package server

import (
	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	//mysql and mssql go libraries
	_ "github.com/denisenkom/go-mssqldb"
)

const activeSessionsQuery = `select distinct User_Name, Time_Logged_On from PA_COMPANY_CODE cc1 with (nolock)
								join (select distinct Operator_ID from PA_OPERATOR_MASTER with(nolock)) om
								on  cc1.User_Id = om.Operator_Id 
								and Time_Logged_On = (select max(Time_Logged_On) from PA_COMPANY_CODE cc2 where cc1.User_Name=cc2.User_Name)`

func (s *Server) getActiveSessions(env string) ([]Session, error) {
	mod, err := kv.NewModifier(s.VaultToken, s.VaultAddr, s.CertPath)
	if err != nil {
		return nil, err
	}
	mod.Env = env
	connInfo, err := mod.ReadData("apiLogins/meta")

	var url, username, password string
	url, ok := connInfo["sessionDB"].(string)
	if !ok {
		return nil, fmt.Errorf("Spectrum database connection not a string or not found")
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

	rows, err := db.Query(activeSessionsQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	var id int = 0
	for rows.Next() {
		var name string
		var loggedIn string

		err := rows.Scan(&name, &loggedIn)
		if err != nil {
			return nil, err
		}

		loggedIn = strings.TrimSpace(loggedIn)
		loc, err := time.LoadLocation("America/Los_Angeles")
		if err != nil {
			return nil, err
		}

		t, err := time.ParseInLocation("01/02/2006 15:04:05", loggedIn, loc)
		if err != nil {
			return nil, err
		}

		sessions = append(sessions, Session{
			ID:        id,
			User:      strings.TrimSpace(name),
			LastLogIn: t.Unix(),
		})
		id++
	}

	return sessions, nil
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

func (s *Server) getVaultSessions(env string) ([]Session, error) {
	mod, err := kv.NewModifier(s.VaultToken, s.VaultAddr, s.CertPath)
	if err != nil {
		return nil, err
	}
	mod.Env = ""

	sessions := []Session{}
	paths, err := mod.List("apiLogins/" + env)
	if paths == nil {
		return nil, fmt.Errorf("Nothing found under apiLogins/" + env)
	}
	if err != nil {
		return nil, err
	}
	mod.Env = env

	// Pass through all registered users
	var id int = 0
	if _, ok := paths.Data["keys"].([]interface{}); ok {
		for id, user := range users {
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
				sessions = append(sessions, Session{
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
