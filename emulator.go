package chip8

import (
	"context"
	"errors"
	"image"
	"image/color"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
)

const (
	FontStartAddress    = 0x50
	ProgramStartAddress = 0x200

	clockTimer time.Duration = time.Second / 60  // 60hz
	clockCPU   time.Duration = time.Second / 700 // 700hz

	width  int = 64
	height int = 32

	area int = width * height
)

var fontSet = []byte{
	0xF0, 0x90, 0x90, 0x90, 0xF0, // 0
	0x20, 0x60, 0x20, 0x20, 0x70, // 1
	0xF0, 0x10, 0xF0, 0x80, 0xF0, // 2
	0xF0, 0x10, 0xF0, 0x10, 0xF0, // 3
	0x90, 0x90, 0xF0, 0x10, 0x10, // 4
	0xF0, 0x80, 0xF0, 0x10, 0xF0, // 5
	0xF0, 0x80, 0xF0, 0x90, 0xF0, // 6
	0xF0, 0x10, 0x20, 0x40, 0x40, // 7
	0xF0, 0x90, 0xF0, 0x90, 0xF0, // 8
	0xF0, 0x90, 0xF0, 0x10, 0xF0, // 9
	0xF0, 0x90, 0xF0, 0x90, 0x90, // A
	0xE0, 0x90, 0xE0, 0x90, 0xE0, // B
	0xF0, 0x80, 0x80, 0x80, 0xF0, // C
	0xE0, 0x90, 0x90, 0x90, 0xE0, // D
	0xF0, 0x80, 0xF0, 0x80, 0xF0, // E
	0xF0, 0x80, 0xF0, 0x80, 0x80, // F
}

var keyMap = map[fyne.KeyName]uint8{
	fyne.Key1: 0x1, fyne.Key2: 0x2, fyne.Key3: 0x3, fyne.Key4: 0xC,
	fyne.KeyQ: 0x4, fyne.KeyW: 0x5, fyne.KeyE: 0x6, fyne.KeyR: 0xD,
	fyne.KeyA: 0x7, fyne.KeyS: 0x8, fyne.KeyD: 0x9, fyne.KeyF: 0xE,
	fyne.KeyZ: 0xA, fyne.KeyX: 0x0, fyne.KeyC: 0xB, fyne.KeyV: 0xF,
}

type delay struct {
	timer uint8
}

func (d *delay) Value() uint8 {
	return d.timer
}

func (d *delay) Add(n uint8) {
	d.timer += n
}

func (d *delay) Dec() {
	if d.timer > 0 {
		d.timer--
	}
}

type sound struct {
	timer uint8
	beep  Beep
}

func (s *sound) Value() uint8 {
	return s.timer
}

func (s *sound) Add(n uint8) {
	current := s.timer
	s.timer += n

	if current == 0 && s.timer > 0 {
		err := s.beep.Start(context.Background())
		if err != nil {
			log.Printf("error starting sound: %v\n", err)
		}
	}
}

func (s *sound) Dec() {
	if s.timer > 0 {
		s.timer--
	}

	if s.timer == 0 {
		s.beep.Stop()
	}
}

type node struct {
	value uint16
	next  *node
}

type Emulator struct {
	memory   [4096]byte
	v        [16]byte
	keyState [16]atomic.Bool
	stack    *node
	pc       uint16
	i        uint16
	delay    delay
	sound    sound
	running  atomic.Bool
}

func (e *Emulator) Load(b []byte) error {
	for i := 0; i < len(b); i++ {
		e.memory[ProgramStartAddress+i] = b[i]
	}
	return nil
}

func (e *Emulator) loadFont() {
	for i, symbol := range fontSet {
		e.memory[FontStartAddress+i] = symbol
	}
}

func (e *Emulator) onKeyDown(k *fyne.KeyEvent) {
	if hex, ok := keyMap[k.Name]; ok {
		e.keyState[hex].Store(true)
	}
}

func (e *Emulator) onKeyUp(k *fyne.KeyEvent) {
	if hex, ok := keyMap[k.Name]; ok {
		e.keyState[hex].Store(false)
	}
}

func (e *Emulator) drawSprite(buffer []byte, x, y, height byte) {
	startX := uint16(x) % uint16(width)
	startY := uint16(y) % uint16(height)

	e.v[0xF] = 0
}

func (e *Emulator) Run() error {
	e.loadFont()
	e.pc = ProgramStartAddress

	a := app.New()
	w := a.NewWindow("Chip-8 Emulator")

	e.running.Store(true)

	// 1. Create a back-buffer for the pixel data
	// 2025 Standard: Use image.NewRGBA for high-performance direct pixel access
	buffer := image.NewRGBA(image.Rect(0, 0, width, height))

	// 2. Create the Fyne canvas object from the image buffer
	image := canvas.NewImageFromImage(buffer)
	image.FillMode = canvas.ImageFillStretch  // Scales the 64x32 grid to window size
	image.ScaleMode = canvas.ImageScalePixels // Maintains "pixelated" retro look

	canv, ok := w.Canvas().(desktop.Canvas)
	if !ok {
		return errors.New("emulator cannot be run on mobile")
	}
	canv.SetOnKeyDown(e.onKeyDown)
	canv.SetOnKeyUp(e.onKeyUp)

	w.SetContent(image)
	w.Resize(fyne.NewSize(float32(width*10), float32(height*10))) // 10x scale for visibility

	var wg sync.WaitGroup

	wg.Go(func() {
		lastTimerUpdate := time.Now()

		cpuTicker := time.NewTicker(clockCPU)
		defer cpuTicker.Stop()

		display := [area]byte{}

		var redraw bool

		for range cpuTicker.C {
			if !e.running.Load() {
				break
			}

			if e.pc > 0xFFE {
				log.Println("program runaway")
				break
			}

			high := uint16(e.memory[e.pc])
			low := uint16(e.memory[e.pc+1])

			e.pc += 2

			opcode := (high << 8) | low
			kind := (opcode & 0xF000) >> 12
			x := (opcode & 0x0F00) >> 8
			y := (opcode & 0x00F0) >> 4
			n := opcode & 0x000F
			nn := opcode & 0x00FF
			nnn := opcode & 0x0FFF

			switch kind {
			case 0x0:
				if opcode == 0x00E0 {
					for i := range display {
						display[i] = 0
					}
					redraw = true
				}
			case 0x1:

			case 0x6:

			case 0x7:

			case 0xA:

			case 0xD:
				// DXYN: Draw sprite at (VX, VY) with height N
				cpu.DrawSprite(cpu.V[x], cpu.V[y], byte(n))
				redraw = true
			}
			// Execute Opcode

			for i, val := range display {
				x, y := i%width, i/width
				c := color.Black
				if val == 1 {
					c = color.White
				}
				buffer.Set(x, y, c) // Directly sets pixels in the buffer
			}

			if redraw {
				fyne.Do(func() {
					image.Refresh()
				})
				redraw = false
			}

			if time.Since(lastTimerUpdate) >= clockTimer {
				e.sound.Dec()
				e.delay.Dec()
				lastTimerUpdate = time.Now()
			}
		}
	})

	w.ShowAndRun()
	e.running.Store(false)
	wg.Wait()

	return nil
}
