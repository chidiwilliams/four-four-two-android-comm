package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/google/gousb"
	"io"
	"log"
)

func RecoverDo(f func(interface{}), g func()) {
	if r := recover(); r != nil {
		f(r)
	} else {
		g()
	}
}

func ReadCommands(r io.Reader, out chan<- *Command) {
	defer RecoverDo(
		func(x interface{}) {
			log.Print("USB Reader terminates due to:", x)
		},
		func() {
			log.Printf("USB Reader terminates normally. This should never happen.")
		},
	)
	defer close(out)

	decoder := json.NewDecoder(r)
	for {
		var cmd Command
		if err := decoder.Decode(&cmd); err != nil {
			panic(fmt.Sprintf("JSON decoder error: %v", err))
		}
		out <- &cmd
	}
}

func WriteReports(ep *gousb.OutEndpoint, in <-chan interface{}, sent chan<- bool, notify chan<- int, id int) {
	defer RecoverDo(
		func(x interface{}) {
			notify <- id
			log.Print("USB Writer terminates due to:", x)
		},
		func() {
			log.Printf("USB Writer terminates normally")
		},
	)

	for obj := range in {
		var body []byte
		var err error

		if obj == nil {
			if body, err = json.Marshal(struct{}{}); err != nil {
				panic(err)
			}
		} else {
			if body, err = json.Marshal(obj); err != nil {
				panic(err)
			}
		}

		length := len(body)

		log.Printf("Writing USB Payload (%d bytes): %s", length, sliceStrSafe(string(body), 100))

		header := make([]byte, 8)
		binary.BigEndian.PutUint32(header, uint32(length))

		if _, err = ep.Write(header); err != nil {
			panic(err)
		}

		if _, err = ep.Write(body); err != nil {
			panic(err)
		}

		sent <- true
	}
}
