package main

import (
	"bufio"
	"encoding/base64"
	"io/ioutil"
	"log"
	"os"
)

type finalImageCmd struct {
	locations []string
	scores    []string
}

type finalImageResult struct {
	images []string
	scores []string
}

func handleFinalImage(in <-chan finalImageCmd, out chan<- finalImageResult, notify chan<- int, id int) {
	defer recoverDo(
		func(x interface{}) {
			notify <- id
			log.Printf("Final image handler terminates due to: %s", x)
		},
		func() {
			log.Printf("Final image handler terminates normally")
		},
	)

	for {
		select {
		case cmd, ok := <-in:
			if !ok {
				return
			}

			images := make([]string, len(cmd.locations))

			for i, loc := range cmd.locations {
				encoded, err := imageFileToBase64(loc)
				if err != nil {
					log.Printf("Error in final image handler: %v", err)
					continue
				}
				images[i] = encoded
			}

			log.Printf("Sending %d images and %d scores", len(images), len(cmd.scores))
			out <- finalImageResult{images: images, scores: cmd.scores}
		}
	}
}

func imageFileToBase64(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		log.Printf("Error in final image handler: %v", err)
		return "", err
	}

	r := bufio.NewReader(file)
	content, err := ioutil.ReadAll(r)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(content), nil
}
