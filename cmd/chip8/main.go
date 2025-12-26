package main

import (
	"chip8"
	"log"
)

func main() {
	var e chip8.Emulator

	err := e.Run()
	if err != nil {
		log.Fatal(err)
	}
}
