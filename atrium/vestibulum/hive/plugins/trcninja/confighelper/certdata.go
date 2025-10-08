package confighelper

import (
	"html/template"
	"io/ioutil"
	"strings"
	"text/template/parse"

	etlcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/core"
)

func GetHost(templatePath string) (string, error) {
	host := ""
	templateFile, err := ioutil.ReadFile(templatePath)
	newTemplate := string(templateFile)
	if err != nil {
		return "", err
	}
	// Parse template
	t := template.New("template")
	theTemplate, err := t.Parse(newTemplate)
	if err != nil {
	}
	commandList := theTemplate.Tree.Root
	for _, node := range commandList.Nodes {
		if node.Type() == parse.NodeAction {
			fields := node.(*parse.ActionNode).Pipe
			for _, arg := range fields.Cmds[0].Args {
				templateParameter := strings.ReplaceAll(arg.String(), "\\\"", "\"")
				if strings.Contains(templateParameter, "~") {
					etlcore.LogError("Unsupported parameter name character ~: " + templateParameter)
					return "", err
				}
				if templateParameter == ".certHost" {
					host = fields.Cmds[0].Args[2].String()
					host = strings.ReplaceAll(host, "\"", "")
					goto hostFound
				}
			}
		}
	}
hostFound:
	return host, err
}
