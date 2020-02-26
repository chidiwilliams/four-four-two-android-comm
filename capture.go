package main

import (
	"bufio"
	"log"
	"os/exec"
)

const (
	CaptureStart = 1 << iota
)

type CaptureCmd struct {
	mode int
	args []string
}

func Capture(in <-chan CaptureCmd, out chan<- string, notify chan<- int, id int) {
	defer RecoverDo(
		func(x interface{}) {
			notify <- id
			log.Printf("Capture terminates due to: %s", x)
		},
		func() {
			log.Printf("Capture terminates normally")
		},
	)

	for {
		select {
		case cmd, ok := <-in:
			if !ok {
				return
			}

			switch cmd.mode {
			case CaptureStart:
				err := RunStdout(out, moonshotExecutableFilePath, cmd.args...)
				if err != nil {
					panic(err)
				}
			}
		}
	}
}

func RunStdout(out chan<- string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	log.Printf("Command start: running %s with args %v\n", name, args)
	defer log.Printf("Command end")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	cmd.Stderr = cmd.Stdout

	scanner := bufio.NewScanner(stdout)
	scanner.Split(bufio.ScanLines)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 5*1024*1024)

	if err := cmd.Start(); err != nil {
		return err
	}

	for scanner.Scan() {
		out <- scanner.Text()
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return cmd.Wait()
}
