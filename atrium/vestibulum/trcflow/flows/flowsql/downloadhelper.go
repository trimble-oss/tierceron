package flowsql

func GetDownloadUpsert(downloadHandle map[string]interface{}, dbName string, tableName string, colID1 string, colID2 string) map[string]interface{} {
	if len(downloadHandle) == 5 { //A query was made not containing defaults yet.
		downloadHandle["type"] = ""
		downloadHandle["targetBlob"] = ""
		downloadHandle["targetSha256"] = ""
		downloadHandle["targetLength"] = ""
	}

	downloadHandle = EscapeForQuery(downloadHandle)

	sqlstr := map[string]interface{}{
		"TrcQuery": "INSERT INTO " + dbName + `.` + tableName + `(` + colID1 + `, ` + colID2 + `, type, targetBlob, targetSha256, targetLength, loaded, changed, lastModified) VALUES ('` +
			downloadHandle[colID1].(string) + `','` +
			downloadHandle[colID2].(string) + `','` +
			downloadHandle["type"].(string) + `','` +
			downloadHandle["targetBlob"].(string) + `','` +
			downloadHandle["targetSha256"].(string) + `','` +
			downloadHandle["targetLength"].(string) + `','` +
			downloadHandle["loaded"].(string) + `','` +
			downloadHandle["changed"].(string) + `','` +
			downloadHandle["lastModified"].(string) + `')` +
			` ON DUPLICATE KEY UPDATE ` +
			`` + colID1 + ` = VALUES(` + colID1 + `), ` + colID2 + ` = VALUES(` + colID2 + `), type = VALUES(type), targetBlob = VALUES(targetBlob), targetSha256 = VALUES(targetSha256), targetLength = VALUES(targetLength), loaded = VALUES(loaded), changed = VALUES(changed), lastModified = VALUES(lastModified)`,
		"TrcChangeId":  []string{downloadHandle[colID1].(string), downloadHandle[colID2].(string)},
		"TrcChangeCol": []string{colID1, colID2},
	}

	return sqlstr
}
