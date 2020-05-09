package main

import (
	"log"
	"strings"
)

var (
	moonshotExecutableFilePath = "./MSFP"

	usbInCmdActionStart   = "start"
	usbInCmdActionCapture = "capture"

	outCmdTypeStart   = "start"
	outCmdTypePreview = "preview"
)

type data map[string]interface{}

type outCmd struct {
	Type string `json:"type"`
	Data data   `json:"data"`
}

func newOutCmd(cmdType string, data data) *outCmd {
	return &outCmd{Type: cmdType, Data: data}
}

func main() {
	stack := openAccessoryModeStack()
	defer func() {
		if err := stack.close(); err != nil {
			log.Println(err)
		}
	}()

	interact(stack)
}

func interact(stack *accessoryModeStack) {
	defer recoverDo(
		func(x interface{}) {
			log.Println("Interactor exit due to:", x)
		},
		func() {
			log.Println("Interactor exit normally")
		},
	)

	usbIn := make(chan *command)
	go readCommands(stack.ReadStream, usbIn)
	defer close(usbIn)

	notifyIn := make(chan int, 9)
	const (
		usbWriterId = 1 << iota
		captureId
		finalImageHandlerId
	)

	usbOut, sentIn := make(chan interface{}, 9), make(chan bool)
	go writeReports(stack.OutEndpoint, usbOut, sentIn, notifyIn, usbWriterId)
	defer close(usbOut)

	captureControlOut, captureResultsIn := make(chan captureCmd, 9), make(chan string)
	go capture(captureControlOut, captureResultsIn, notifyIn, captureId)
	defer close(captureControlOut)

	finalImageControlOut, finalImageResultsIn := make(chan finalImageCmd, 9), make(chan finalImageResult)
	go handleFinalImage(finalImageControlOut, finalImageResultsIn, notifyIn, finalImageHandlerId)
	defer close(finalImageControlOut)

	usbWriterPending := 0

	for {
		select {
		case cmd, ok := <-usbIn:
			if !ok {
				log.Println("USB Reader died. I am dying too.")
				return
			}

			log.Printf("USB command received: %v", cmd)

			switch cmd.Action {
			case usbInCmdActionStart:
				usbOut <- newOutCmd(outCmdTypeStart, nil)
			case usbInCmdActionCapture:
				captureControlOut <- captureCmd{captureStart, cmd.Args}
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
				usbOut <- newOutCmd(outCmdTypePreview, data{"image": captureResult[6:]})
			} else if len(captureResult) > 17 && captureResult[:17] == "Processed images:" {
				locations, scores := make([]string, 0), make([]string, 0)

				w := strings.Split(captureResult, " ")
				log.Printf("w: %+v\n", w)
				for _, l := range w[2:] {
					m := strings.Split(l, ",")
					locations = append(locations, m[0])
					scores = append(scores, m[1])
				}

				cmd := finalImageCmd{locations: locations, scores: scores}
				log.Printf("Sending final image command: %+v", cmd)
				finalImageControlOut <- cmd
			}

		case finalImageRes := <-finalImageResultsIn:
			usbOut <- newOutCmd("fingerprint", data{"images": finalImageRes.images, "scores": finalImageRes.scores})
		}
	}
}

func sliceStrSafe(s string, i int) string {
	if len(s) > i {
		return s[0:i]
	}
	return s
}
