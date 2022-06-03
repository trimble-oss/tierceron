//go:build !tc
// +build !tc

package coreopts

// Which tables synced..
func GetSyncedTables() []string {
	return []string{}
}

func GetFolderPrefix() string {
	return "trc"
}
