package testutils

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/plotly/plotbot/util"
)

type MockRunner struct {
	Jobs        []util.Searchable
	ParseVars   func(string, ...string) []string
	TestCmdName string
}

func ClearMockRunner(r *MockRunner) {
	r.Jobs = []util.Searchable{}
}

// see https://npf.io/2015/06/testing-exec-command/
func (r *MockRunner) Run(c string, s ...string) *exec.Cmd {

	allc := append([]string{c}, s...)
	r.Jobs = append(r.Jobs, util.Searchable(allc))

	testcmd := r.TestCmdName
	if testcmd == "" {
		testcmd = "TestCmdProcess"
	}

	cs := []string{fmt.Sprintf("-test.run=%s", testcmd)}
	cmd := exec.Command(os.Args[0], cs...)

	env := []string{}
	if r.ParseVars != nil {
		env = r.ParseVars(c, s...)
	}
	env = append(env, "GO_WANT_CMD_PROCESS=1")
	cmd.Env = env

	return cmd
}
