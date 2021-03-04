package server

import (
	"crypto/sha512"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"Vault.Whoville/vaulthelper/kv"
	"golang.org/x/crypto/pbkdf2"

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

	rows, err := db.Query(GetAuthLoginQuery(), sql.Named("Id", operatorId))
	if err != nil {
		return false, "", err
	}
	defer rows.Close()

	for rows.Next() {
		var operatorId string
		var operatorName string
		var passwordHash string
		var saltEncoded string
		var iterationCount int

		// Operator_ID, Password_Hash, Salt, Iteration_Count
		err := rows.Scan(&operatorId, &operatorName, &passwordHash, &saltEncoded, &iterationCount)
		if err != nil {
			return false, "", err
		}
		salt, err := base64.StdEncoding.DecodeString(saltEncoded)
		if err != nil {
			return false, "", err
		}
		operatorPasswordByteArray := pbkdf2.Key([]byte(operatorPassword), salt, iterationCount, 64, sha512.New)
		operatorPasswordHash := base64.StdEncoding.EncodeToString(operatorPasswordByteArray)

		if string(operatorPasswordHash) == passwordHash {
			return true, operatorName, nil
		} else {
			return false, "", errors.New("Invalid password")
		}

	}

	return false, "", errors.New("Invalid password")
}

func (s *Server) getActiveSessions(env string) ([]Session, error) {
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

	rows, err := db.Query(GetActiveSessionQuery())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	var id int
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
	mod, err := kv.NewModifier(s.VaultToken, s.VaultAddr, "nonprod", nil)
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
