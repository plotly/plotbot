package deployer

import (
	"fmt"
	"strings"

	"github.com/plotly/plotbot"
)

type DeployParams struct {
	Environment     string
	Branch          string
	Tags            string
	InitiatedBy     string
	From            string
	initiatedByChat *plotbot.Message
}

// ParsedTags returns *default* or user-specified tags
func (p *DeployParams) ParsedTags() string {
	tags := strings.Replace(p.Tags, " ", "", -1)
	if tags == "" {
		tags = "updt_streambed"
	}
	return tags
}

func (p *DeployParams) String() string {
	branch := p.Branch
	if branch == "" {
		branch = "[default]"
	}

	str := fmt.Sprintf("env=%s branch=%s tags=%s", p.Environment, branch, p.ParsedTags())

	str = fmt.Sprintf("%s by %s", str, p.InitiatedBy)

	return str
}
