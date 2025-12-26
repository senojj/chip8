package main

import (
	"emul8"
	"io"
	"log"
	"os"
)

func main() {
	var e emul8.Emulator

	f, err := os.Open("ibm_logo.ch8")
	if err != nil {
		log.Fatal(err)
	}

	b, err := io.ReadAll(f)
	if err != nil {
		log.Fatal(err)
	}

	err = e.Load(b)
	if err != nil {
		log.Fatal(err)
	}

	err = e.Run()
	if err != nil {
		log.Fatal(err)
	}
}
