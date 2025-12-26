package chip8

import (
	"context"
	"errors"
	"image"
	"image/color"
	"log"
	"math/rand/v2"
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

func (d *delay) Set(n uint8) {
	d.timer = n
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

func (s *sound) Set(n uint8) {
	current := s.timer
	s.timer = n

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

func (e *Emulator) drawSprite(buffer []byte, x, y, h byte) {
	startX := uint16(x) & uint16(width-1)
	startY := uint16(y) & uint16(height-1)

	e.v[0xF] = 0 // Reset the collision register.

	for row := uint16(0); row < uint16(h); row++ {
		if startY+row >= uint16(height) {
			// Reached the bottom of the display.
			break
		}

		sprite := e.memory[e.i+row]

		for col := uint16(0); col < 8; col++ {
			if startX+col >= uint16(width) {
				break
			}

			if (sprite & (0x80 >> col)) != 0 {
				index := (startX + col) + ((startY + row) * uint16(width))

				if buffer[index] == 1 {
					// Pixel was already on. This indicates a graphical object collision.
					e.v[0xF] = 1 // Turn on the collision register.
					buffer[index] = 0
				} else {
					buffer[index] = 1
				}
			}
		}
	}
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
				switch opcode {
				case 0x00E0:
					// Clear the screen
					for i := range display {
						display[i] = 0
					}
					redraw = true
				case 0x00EE:
					// Return from subroutine
					item := e.stack
					if item == nil {
						panic("return from empty stack")
					}
					e.pc = item.value
					e.stack = item.next
				default:
					panic("unknown 0x0 opcode")
				}
			case 0x1:
				// Jump to location
				e.pc = nnn
			case 0x2:
				// Call subroutine
				item := &node{
					value: e.pc,
					next:  e.stack,
				}
				e.stack = item
				e.pc = nnn
			case 0x3:
				if e.v[x] == byte(nn) {
					e.pc += 2
				}
			case 0x4:
				if e.v[x] != byte(nn) {
					e.pc += 2
				}
			case 0x5:
				if e.v[x] == e.v[y] {
					e.pc += 2
				}
			case 0x6:
				// Set register
				e.v[x] = byte(nn)
			case 0x7:
				// Add to register
				x := (opcode & 0x0F00) >> 8
				nn := byte(opcode & 0x00FF)
				e.v[x] += nn
			case 0x8:
				switch n {
				case 0x0:
					e.v[x] = e.v[y]
				case 0x1:
					e.v[0xF] = 0
					e.v[x] |= e.v[y]
				case 0x2:
					e.v[0xF] = 0
					e.v[x] &= e.v[y]
				case 0x3:
					e.v[0xF] = 0
					e.v[x] ^= e.v[y]
				case 0x4:
					sum := uint16(e.v[x]) + uint16(e.v[y])
					e.v[0xF] = 0
					if sum > 255 {
						e.v[0xF] = 1
					}
					e.v[x] = byte(sum & 0xFF)
				case 0x5:
					e.v[0xF] = 0
					if e.v[x] >= e.v[y] {
						e.v[0xF] = 1
					}
					e.v[x] -= e.v[y]
				case 0x6:
					e.v[0xF] = e.v[x] & 0x1
					e.v[x] >>= 1
				case 0x7:
					e.v[0xF] = 0
					if e.v[y] >= e.v[x] {
						e.v[0xF] = 1
					}
					e.v[x] = e.v[y] - e.v[x]
				case 0xE:
					e.v[0xF] = (e.v[x] & 0x80) >> 7
					e.v[x] <<= 1
				}
			case 0x9:
				if e.v[x] != e.v[y] {
					e.pc += 2
				}
			case 0xA:
				// Set index
				e.i = nnn
			case 0xB:
				e.pc = nnn + uint16(e.v[0x0])
			case 0xC:
				randomByte := byte(rand.Uint32N(256))
				e.v[x] = randomByte & byte(nn)
			case 0xD:
				e.drawSprite(display[:], e.v[x], e.v[y], byte(n))
				redraw = true
			case 0xE:
				key := e.v[x] & 0x0F

				switch nn {
				case 0x9E:
					if e.keyState[key].Load() {
						e.pc += 2
					}
				case 0xA1:
					if !e.keyState[key].Load() {
						e.pc += 2
					}
				default:
					panic("unknown 0xE opcode")
				}
			case 0xF:
				switch nn {
				case 0x07:
					e.v[x] = e.delay.Value()
				case 0x0A:
					var keyPressed bool

					for i := uint8(0); i < uint8(len(e.keyState)); i++ {
						if e.keyState[i].Load() {
							keyPressed = true
							e.v[x] = i
							break
						}
					}

					if !keyPressed {
						e.pc -= 2
					}
				case 0x15:
					e.delay.Set(e.v[x])
				case 0x18:
					e.sound.Set(e.v[x])
				case 0x1E:
					e.i += uint16(e.v[x])
				case 0x29:
					digit := uint16(e.v[x] & 0x0F)
					e.i = FontStartAddress + (digit * 5)
				case 0x33:
					val := uint32(e.v[x])

					// Double dabble algorithm
					var bcd uint32

					// Iterate 8 times (once for each bit of the input byte)
					for i := 0; i < 8; i++ {
						// 1. Check each BCD nibble. If >= 5, add 3.
						// Hundreds (bits 8-11), Tens (bits 4-7), Ones (bits 0-3)
						if (bcd & 0x00F) >= 5 {
							bcd += 3
						}
						if (bcd & 0x0F0) >= 0x050 {
							bcd += 0x030
						}
						if (bcd & 0xF00) >= 0x500 {
							bcd += 0x300
						}

						// 2. Shift BCD left by 1, and pull in the next bit from 'val'
						bcd = (bcd << 1) | ((val >> (7 - i)) & 1)
					}

					e.memory[e.i] = byte((bcd >> 8) & 0xF)   // Hundreds
					e.memory[e.i+1] = byte((bcd >> 4) & 0xF) // Tens
					e.memory[e.i+2] = byte(bcd & 0xF)        // Ones
				default:
					panic("unknown 0xF opcode")
				}
			}

			if redraw {
				for i, val := range display {
					x, y := i%width, i/width
					c := color.Black
					if val == 1 {
						c = color.White
					}
					buffer.Set(x, y, c) // Directly sets pixels in the buffer
				}

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
