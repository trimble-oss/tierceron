// Package lib contains the types for schema 'fieldtechservice'.
package lib

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	"errors"
	"io/ioutil"
	"time"

	"github.com/jmoiron/sqlx"
)

// Tried time.RFC3339 but that doesn't work.
const RFC_ISO_8601 = "2006-01-02 15:04:05 -0700 MST"

func CheckTableChanges(db XODB, tableFromVault map[string]interface{}) (int, error) {
	changed := 0
	if db != nil {
		var err error
		// sql query
		sqlstr := `select 1 from fieldtechservice.MysqlFile where MysqlFilePath="` + mysqlFileFromVault["MysqlFilePath"].(string) + `" and lastModified > "` + mysqlFileFromVault["lastModified"].(time.Time).String() + `"`

		// run query
		XOLog(sqlstr)

		rows, err := db.Query(sqlstr)
		if err != nil {
			return -1, err
		}

		defer rows.Close()

		for rows.Next() {
			err = rows.Scan(&changed)
			if err != nil {
				return -1, err
			}
		}
		//Any error encountered during iteration
		err = rows.Err()
		if err != nil {
			return -1, err
		}
	}

	return changed, nil
}

func GetLocalTable(databaseName string, tenantTable string) map[string]string {
	return map[string]string{"TrcQuery": "SELECT * FROM " + databaseName + "." + tenantTable}
}

func GetTables(db XODB, changedFilePaths []string, existingFilePaths []string) ([]*MysqlFile, error) {
	var mysqlFiles []*MysqlFile
	if db != nil {
		if len(changedFilePaths) > 0 {
			const sqlstr = `SELECT * FROM fieldtechservice.MysqlFile where MysqlFilePath in (?)`
			XOLog(sqlstr)
			query, args, err := sqlx.In(sqlstr, changedFilePaths)
			if err != nil {
				return nil, err
			}

			rows, err := db.Query(query, args...)
			// run query
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			for rows.Next() {
				t := MysqlFile{
					_exists: true,
				}
				err = rows.Scan(&t.MysqlFilePath, &t.MysqlFileContent, &t.LastModified)
				if err != nil {
					return nil, err
				}
				t.MysqlFileContent, err = compressBytes(t.MysqlFileContent) //We compress MysqlFileContent here so that it can fit inside vault.
				if err != nil {
					return nil, errors.New("Could not compress incoming MysqlFileContent")
				}

				mysqlFiles = append(mysqlFiles, &t)
			}
			//Any error encountered during iteration
			err = rows.Err()
			if err != nil {
				return nil, err
			}
			if err != nil {
				return nil, err
			}
		}

		if len(existingFilePaths) == 0 && len(changedFilePaths) == 0 {
			existingFilePaths = append(existingFilePaths, "")
		}

		if len(existingFilePaths) > 0 {
			//Find any new rows
			newMysqlFiles, err := GetNewMysqlFiles(db, existingFilePaths) //Grabs rows that didn't get changed
			if err != nil {
				return nil, err
			} else {
				mysqlFiles = append(mysqlFiles, newMysqlFiles...)
			}
		}
	}
	return mysqlFiles, nil
}

func GetNewTableRows(db XODB, changedFilePaths []string) ([]*MysqlFile, error) {
	var mysqlFiles []*MysqlFile
	if db != nil {
		const sqlstr = `SELECT * FROM fieldtechservice.MysqlFile where MysqlFilePath not in (?)`
		XOLog(sqlstr)
		query, args, err := sqlx.In(sqlstr, changedFilePaths)
		if err != nil {
			return nil, err
		}

		rows, err := db.Query(query, args...)
		// run query
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			t := MysqlFile{
				_exists: true,
			}
			err = rows.Scan(&t.MysqlFilePath, &t.MysqlFileContent, &t.LastModified)
			if err != nil {
				return nil, err
			}
			t.MysqlFileContent, err = compressBytes(t.MysqlFileContent) //We compress MysqlFileContent here so that it can fit inside vault.
			if err != nil {
				return nil, errors.New("Could not compress incoming MysqlFileContent")
			}

			mysqlFiles = append(mysqlFiles, &t)
		}
		//Any error encountered during iteration
		err = rows.Err()
		if err != nil {
			return nil, err
		}
		if err != nil {
			return nil, err
		}
	}

	return mysqlFiles, nil
}

func compressBytes(uncompressed []byte) ([]byte, error) {
	var result bytes.Buffer
	compressor, err := gzip.NewWriterLevel(&result, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	_, err = compressor.Write(uncompressed)
	if err != nil {
		return nil, err
	}

	if err := compressor.Close(); err != nil {
		return nil, err
	}

	return result.Bytes(), nil
}

func UncompressBytes(compressed []byte) ([]byte, error) {
	uncompressor, err := gzip.NewReader(bytes.NewBuffer(compressed))
	if err != nil {
		return nil, err
	}
	result, err := ioutil.ReadAll(uncompressor)
	if err != nil {
		return nil, err
	}

	if err := uncompressor.Close(); err != nil {
		return nil, err
	}

	return result, nil
}

func GetMysqlFileMap(k *MysqlFile) map[string]interface{} {
	m := make(map[string]interface{})
	m["MysqlFilePath"] = k.MysqlFilePath
	m["MysqlFileContent"] = k.MysqlFileContent //Cast as string for comparsion for checking updates
	m["lastModified"] = k.LastModified.Time.Format(RFC_ISO_8601)
	return m
}

func GetMysqlFileMapFromArray(t []interface{}) map[string]interface{} {
	m := make(map[string]interface{})
	m["MysqlFilePath"] = t[0]
	m["MysqlFileContent"] = t[1]
	m["lastModified"] = t[2]
	return m
}

func GetMysqlFileMapFromMap(m map[string]interface{}) *MysqlFile {
	t := MysqlFile{
		_exists: true,
	}
	t.MysqlFilePath = m["MysqlFilePath"].(string)
	t.MysqlFileContent = []byte(m["MysqlFileContent"].([]uint8))

	if _, ok := m["lastModified"].(string); ok {
		if m["lastModified"].(string) == "" {
			t.LastModified = sql.NullTime{Time: time.Time{}, Valid: false}
		} else {
			var err error
			t.LastModified = sql.NullTime{Valid: true}
			t.LastModified.Time, err = time.Parse(RFC_ISO_8601, m["lastModified"].(string))
			if err != nil {
				t.LastModified.Valid = false
			}
		}
	} else {
		t.LastModified = sql.NullTime{Time: m["lastModified"].(time.Time), Valid: true}
	}
	return &t
}
