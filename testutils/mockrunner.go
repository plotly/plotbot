package testutils

import (
	"fmt"
	"os"
	"os/exec"
)

type MockRunner struct {
	jobs     [][]string
	exitCode int64
}

func (r MockRunner) Run(c string, s ...string) *exec.Cmd {

	allc := append([]string{c}, s...)
	r.jobs = append(r.jobs, allc)

	cs := []string{"-test.run=TestCmdProcess", "--"}
	cs = append(cs, allc...)

	exitCodeVar := fmt.Sprintf("GO_CMD_PROCESS_EXIT_CODE=%s", r.exitCode)

	// see https://npf.io/2015/06/testing-exec-command/
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_CMD_PROCESS=1", exitCodeVar}

	return cmd
}
