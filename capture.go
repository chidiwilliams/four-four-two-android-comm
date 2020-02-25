package main

import (
	"bufio"
	"log"
	"os/exec"
)

const (
	CaptureStart = 1 << iota
)

func Capture(in <-chan int, out chan<- string, notify chan<- int, id int) {
	defer RecoverDo(
		func(x interface{}) {
			notify <- id
			log.Print("Capture terminates due to:", x)
		},
		func() {
			log.Printf("Capture terminates normally")
		},
	)

	for {
		select {
		case x, ok := <-in:
			if !ok {
				return
			}

			switch x {
			case CaptureStart:
				err := RunStdout(out, moonshotExecutableFilePath, "LEFT")
				if err != nil {
					out <- err.Error()
				}
			}
		}
	}
}

func RunStdout(out chan<- string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	log.Printf("Running %s with args %v\n", name, args)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	cmd.Stderr = cmd.Stdout

	scanner := bufio.NewScanner(stdout)
	scanner.Split(bufio.ScanLines)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 5*1024*1024)

	log.Print("About to start exec")

	if err := cmd.Start(); err != nil {
		return err
	}
	log.Print("Started exec")

	for scanner.Scan() {
		text := scanner.Text()
		out <- text
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	log.Print("End of capture scanning")
	return cmd.Wait()
}
