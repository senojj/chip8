package main

import (
	"chip8"
	"io"
	"log"
	"os"
)

func main() {
	var e chip8.Emulator

	f, err := os.Open("ibm_logo.ch8")
	if err != nil {
		log.Fatal(err)
	}

	b, err := io.ReadAll(f)
	if err != nil {
		log.Fatal(err)
	}

	e.Load(b)

	err = e.Run()
	if err != nil {
		log.Fatal(err)
	}
}
