package exec

import (
	"bufio"
	"os/exec"
)

// RunStdout runs the named program with the given arguments and pushes each line from
// stdout to the out channel
func RunStdout(out chan<- string, name string, args ...string) error {
	cmd := exec.Command(name, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Split(bufio.ScanLines)

	if err := cmd.Start(); err != nil {
		return err
	}

	for scanner.Scan() {
		text := scanner.Text()
		out <- text
	}

	return cmd.Wait()
}
