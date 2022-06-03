//go:build !testflow
// +build !testflow

package coreopts

// Which tables synced..
func GetSyncedTables() []string {
	return []string{}
}

func GetFolderPrefix() string {
	return "trc"
}
