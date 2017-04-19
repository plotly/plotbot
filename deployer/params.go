package deployer

import (
	"fmt"
	"io/ioutil"
	"regexp"

	"github.com/plotly/plotbot"
)

type DeployParams struct {
	Playbook        string
	Environment     string
	Branch          string
	Tags            string
	InitiatedBy     string
	From            string
	initiatedByChat *plotbot.Message
	Confirm         bool
}

func (p *DeployParams) String() string {
	branch := p.Branch
	if branch == "" {
		branch = "[default]"
	}

	str := fmt.Sprintf("env=%s branch=%s tags=%s", p.Environment, branch, p.Tags)

	str = fmt.Sprintf("%s by %s", str, p.InitiatedBy)

	return str
}

var playbookRegex = regexp.MustCompile(`^playbook_(stage|prod)_(.*).yml$`)

func listAllowedPlaybooks(path string) ([]string, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}
	playbooks := []string{}
	for _, file := range files {
		if match := playbookRegex.FindStringSubmatch(file.Name()); match != nil {
			playbooks = append(playbooks, match[2])
		}
	}

	return playbooks, nil
}
