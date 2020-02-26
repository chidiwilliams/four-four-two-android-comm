package main

import (
	"log"
)

var (
	moonshotExecutableFilePath = "./MSFP"

	usbInCmdActionStart   = "start"
	usbInCmdActionCapture = "capture"

	outCmdTypeStart       = "start"
	outCmdTypePreview     = "preview"
	outCmdTypeScores      = "scores"
	outCmdTypeFingerprint = "fingerprint"
)

type data map[string]interface{}

type outCmd struct {
	Type string `json:"type"`
	Data data   `json:"data"`
}

func NewOutCmd(cmdType string, data data) *outCmd {
	return &outCmd{Type: cmdType, Data: data}
}

func main() {
	stack := OpenAccessoryModeStack()
	defer stack.Close()

	Interact(stack)
}

func Interact(stack *AccessoryModeStack) {
	defer RecoverDo(
		func(x interface{}) {
			log.Println("Interactor exit due to:", x)
		},
		func() {
			log.Println("Interactor exit normally")
		},
	)

	usbIn := make(chan *Command)
	go ReadCommands(stack.ReadStream, usbIn)
	defer close(usbIn)

	notifyIn := make(chan int, 9)
	const (
		usbWriterId = 1 << iota
		captureId
	)

	usbOut, sentIn := make(chan interface{}, 9), make(chan bool)
	go WriteReports(stack.OutEndpoint, usbOut, sentIn, notifyIn, usbWriterId)
	defer close(usbOut)

	captureControlOut, captureResultsIn := make(chan CaptureCmd, 9), make(chan string)
	go Capture(captureControlOut, captureResultsIn, notifyIn, captureId)
	defer close(captureControlOut)

	usbWriterPending := 0

	for {
		select {
		case command, ok := <-usbIn:
			if !ok {
				log.Println("USB Reader died. I am dying too.")
				return
			}

			log.Printf("USB command received: %v", command)

			switch command.Action {
			case usbInCmdActionStart:
				usbOut <- NewOutCmd(outCmdTypeStart, nil)
			case usbInCmdActionCapture:
				captureControlOut <- CaptureCmd{CaptureStart, command.Args}
			}

		case <-sentIn:
			usbWriterPending--

		case child := <-notifyIn:
			switch child {
			case usbWriterId:
				log.Println("USB writer died")
			case captureId:
				log.Println("Capture writer died")
			}

		case captureResult := <-captureResultsIn:
			log.Printf("Capture result: %s", sliceStrSafe(captureResult, 50))
			if len(captureResult) > 6 && captureResult[:6] == "image " {
				usbOut <- NewOutCmd(outCmdTypePreview, data{"image": captureResult[6:]})
			} else if captureResult == "Process Complete" {
				usbOut <- NewOutCmd("error", data{"error": "Successful"})
			}
		}
	}
}

func sliceStrSafe(s string, i int) string {
	if len(s) > i {
		return s[0:i]
	}
	return s
}
