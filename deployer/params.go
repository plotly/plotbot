package deployer

import (
	"fmt"
	"io/ioutil"
	"path"
	"regexp"
	"strings"

	"github.com/plotly/plotbot"
)

type DeployParams struct {
	Playbook        string
	Service         string
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

	str := fmt.Sprintf("service=%s env=%s branch=%s tags=%s", p.Service, p.Environment, branch, p.Tags)

	str = fmt.Sprintf("%s by %s", str, p.InitiatedBy)

	return str
}

var playbookRegex = regexp.MustCompile(`^playbook_(stage|prod)_(.*).yml$`)

type Playbook struct {
	Path        string
	Environment string
	Playbook    string
}

type Playbooks []Playbook

func (ps Playbooks) ByEnvironment(Environment string) Playbooks {
	playbooks := Playbooks{}
	for _, p := range ps {
		if p.Environment == Environment {
			playbooks = append(playbooks, p)
		}
	}
	return playbooks
}

func (ps Playbooks) Playbooks() []string {
	playbooks := []string{}
	for _, p := range ps {
		playbooks = append(playbooks, p.Playbook)
	}
	return playbooks
}

func (ps Playbooks) ToBullets() string {
	return fmt.Sprintf("• %s", strings.Join(ps.Playbooks(), "\n• "))
}

func listAllowedPlaybooks(filepath string) (Playbooks, error) {
	files, err := ioutil.ReadDir(filepath)
	if err != nil {
		return nil, err
	}
	playbooks := Playbooks{}
	for _, file := range files {
		if match := playbookRegex.FindStringSubmatch(file.Name()); match != nil {
			playbooks = append(playbooks, Playbook{
				Playbook:    match[2],
				Path:        path.Join(filepath, file.Name()),
				Environment: match[1],
			})
		}
	}

	return playbooks, nil
}
