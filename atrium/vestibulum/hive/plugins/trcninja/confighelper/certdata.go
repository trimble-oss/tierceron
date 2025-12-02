package confighelper

import (
	"errors"
	"html/template"
	"io/ioutil"
	"strings"
	"text/template/parse"

	etlcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/core"
)

func GetHost(templatePath string) (string, error) {
	host := ""

	if templatePath == "" {
		return "", errors.New("templatePath is empty")
	}

	templateFile, err := ioutil.ReadFile(templatePath)
	if err != nil {
		return "", err
	}

	newTemplate := string(templateFile)
	if newTemplate == "" {
		return "", errors.New("template file is empty")
	}

	// Parse template
	t := template.New("template")
	theTemplate, err := t.Parse(newTemplate)
	if err != nil {
		return "", err
	}

	// Defensive: Check if Tree or Root is nil
	if theTemplate.Tree == nil || theTemplate.Tree.Root == nil {
		return "", errors.New("template tree or root is nil")
	}

	commandList := theTemplate.Tree.Root
	for _, node := range commandList.Nodes {
		if node == nil {
			continue
		}
		if node.Type() == parse.NodeAction {
			actionNode, ok := node.(*parse.ActionNode)
			if !ok || actionNode.Pipe == nil {
				continue
			}
			fields := actionNode.Pipe
			if len(fields.Cmds) == 0 || len(fields.Cmds[0].Args) == 0 {
				continue
			}
			for _, arg := range fields.Cmds[0].Args {
				templateParameter := strings.ReplaceAll(arg.String(), "\\\"", "\"")
				if strings.Contains(templateParameter, "~") {
					etlcore.LogError("Unsupported parameter name character ~: " + templateParameter)
					return "", errors.New("unsupported parameter name character ~: " + templateParameter)
				}
				if templateParameter == ".certHost" {
					if len(fields.Cmds[0].Args) < 3 {
						return "", errors.New("not enough arguments for certHost")
					}
					host = fields.Cmds[0].Args[2].String()
					host = strings.ReplaceAll(host, "\"", "")
					goto hostFound
				}
			}
		}
	}
hostFound:
	return host, nil
}
