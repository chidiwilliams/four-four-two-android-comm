package main

import (
	"fmt"
)

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
		case cmd, ok := <-usbIn:
			fmt.Printf("%+v, %t", cmd, ok)
			usbOut <- map[string]string{"report": "this is working!"}
		}
	}
}
