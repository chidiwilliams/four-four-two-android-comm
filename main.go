package main

import (
	"fmt"
	"os/exec"
)

const (
	moonshotExecutableFilePath = "MSFP"

	usbInCmdActionStart       = "start"
	usbInCmdActionCaptureLeft = "captureLeft"

	outCmdTypeStart = "start"
)

type outCmd struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

func NewOutCmd(cmdType string, data map[string]interface{}) *outCmd {
	return &outCmd{Type: cmdType, Data: data}
}

func main() {
	stack := OpenAccessoryModeStack()
	defer stack.Close()

	usbIn := make(chan *Command)
	go ReadCommands(stack.ReadStream, usbIn)

	notifyIn := make(chan int, 9)
	usbOut, sentIn := make(chan interface{}, 9), make(chan bool)
	go WriteReports(stack.OutEndpoint, usbOut, sentIn, notifyIn, 1)

	for {
		select {
		case cmd, _ := <-usbIn:
			switch cmd.Action {
			case usbInCmdActionStart:
				usbOut <- NewOutCmd(outCmdTypeStart, nil)
			case usbInCmdActionCaptureLeft:
				if err := run442Command("LEFT"); err != nil {
					fmt.Errorf("%+v", err)
				}
			}
		}
	}
}

func run442Command(args ...string) error {
	cmd := exec.Command(moonshotExecutableFilePath, args...)

	_, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	return nil
}
