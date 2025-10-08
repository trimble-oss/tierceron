package util

import "fmt"

const HELP_TEXT = `Echo example help..
`

func GetHelp() string {
	return fmt.Sprintf("{\"text\": \"%s\"}", HELP_TEXT)
}
