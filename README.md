# Emul8
A chip-8 emulator/interpreter

## Building
To install the Fyne dependencies, follow instructions at https://docs.fyne.io/started/quick

To install the PortAudio dependency, follow the download link here https://files.portaudio.com/download.html or check your package manager.

After the necessary dependencies are installed, run the following command:
```
make build
```
This will create a binary at ./bin/emul8

## Running
Running the chip-8 emulator requires a chip-8 program. There are many such programs that can be found all around the internet. This emulator aims to support most older chip-8 programs.
```
./bin/emul8 some_rom.ch8
```
