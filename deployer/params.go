package deployer

import (
	"fmt"

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
