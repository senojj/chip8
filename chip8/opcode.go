package chip8

import (
	"math/rand/v2"
)

type OpCode interface {
	Execute(*Processor, *uint8)
}

type ClearScreen struct{}

func (o *ClearScreen) Execute(cpu *Processor, info *uint8) {
	for i := range cpu.display {
		cpu.display[i] = 0
	}
	*info |= Redraw
}

type SubroutineCall struct {
	nnn uint16
}

func (o *SubroutineCall) Execute(cpu *Processor, _ *uint8) {
	item := &node{
		value: cpu.pc,
		next:  cpu.stack,
	}
	cpu.stack = item
	cpu.pc = o.nnn
}

type SubroutineReturn struct{}

func (o *SubroutineReturn) Execute(cpu *Processor, _ *uint8) {
	item := cpu.stack
	if item == nil {
		panic("return from empty stack")
	}
	cpu.pc = item.value
	cpu.stack = item.next
}

type JumpToLocation struct {
	nnn uint16
}

func (o *JumpToLocation) Execute(cpu *Processor, _ *uint8) {
	cpu.pc = o.nnn
}

type JumpWithOffset struct {
	nnn uint16
}

func (o *JumpWithOffset) Execute(cpu *Processor, _ *uint8) {
	cpu.pc = o.nnn + uint16(cpu.v[0x0])
}

type StepIfXEqualsNN struct {
	x  uint16
	nn uint16
}

func (o *StepIfXEqualsNN) Execute(cpu *Processor, _ *uint8) {
	if cpu.v[o.x] == byte(o.nn) {
		cpu.pc += 2
	}
}

type StepIfXNotEqualsNN struct {
	x  uint16
	nn uint16
}

func (o *StepIfXNotEqualsNN) Execute(cpu *Processor, _ *uint8) {
	if cpu.v[o.x] != byte(o.nn) {
		cpu.pc += 2
	}
}

type StepIfXEqualsY struct {
	x uint16
	y uint16
}

func (o *StepIfXEqualsY) Execute(cpu *Processor, _ *uint8) {
	if cpu.v[o.x] == cpu.v[o.y] {
		cpu.pc += 2
	}
}

type StepIfXNotEqualsY struct {
	x uint16
	y uint16
}

func (o *StepIfXNotEqualsY) Execute(cpu *Processor, _ *uint8) {
	if cpu.v[o.x] != cpu.v[o.y] {
		cpu.pc += 2
	}
}

type SetXToNN struct {
	x  uint16
	nn uint16
}

func (o *SetXToNN) Execute(cpu *Processor, _ *uint8) {
	cpu.v[o.x] = byte(o.nn)
}

type AddNNToX struct {
	x  uint16
	nn uint16
}

func (o *AddNNToX) Execute(cpu *Processor, _ *uint8) {
	cpu.v[o.x] += byte(o.nn)
}

type SetXToY struct {
	x uint16
	y uint16
}

func (o *SetXToY) Execute(cpu *Processor, _ *uint8) {
	cpu.v[o.x] = cpu.v[o.y]
}

type OrXY struct {
	x uint16
	y uint16
}

func (o *OrXY) Execute(cpu *Processor, _ *uint8) {
	// This operation traditionally resets the carry flag.
	cpu.v[CarryFlag] = 0
	cpu.v[o.x] |= cpu.v[o.y]
}

type AndXY struct {
	x uint16
	y uint16
}

func (o *AndXY) Execute(cpu *Processor, _ *uint8) {
	// This operation traditionally resets the carry flag.
	cpu.v[CarryFlag] = 0
	cpu.v[o.x] &= cpu.v[o.y]
}

type XOrXY struct {
	x uint16
	y uint16
}

func (o *XOrXY) Execute(cpu *Processor, _ *uint8) {
	// This operation traditionally resets the carry flag.
	cpu.v[CarryFlag] = 0
	cpu.v[o.x] ^= cpu.v[o.y]
}

type AddXY struct {
	x uint16
	y uint16
}

func (o *AddXY) Execute(cpu *Processor, _ *uint8) {
	sum := uint16(cpu.v[o.x]) + uint16(cpu.v[o.y])
	// This operation traditionally resets the carry flag.
	cpu.v[CarryFlag] = 0
	if sum > 255 {
		cpu.v[CarryFlag] = 1
	}
	cpu.v[o.x] = byte(sum & 0xFF)
}

type SubtractYFromX struct {
	x uint16
	y uint16
}

func (o *SubtractYFromX) Execute(cpu *Processor, _ *uint8) {
	cpu.v[CarryFlag] = 0
	if cpu.v[o.x] >= cpu.v[o.y] {
		cpu.v[CarryFlag] = 1
	}
	cpu.v[o.x] -= cpu.v[o.y]
}

type SubtractXFromY struct {
	x uint16
	y uint16
}

func (o *SubtractXFromY) Execute(cpu *Processor, _ *uint8) {
	cpu.v[CarryFlag] = 0
	if cpu.v[o.y] >= cpu.v[o.x] {
		cpu.v[CarryFlag] = 1
	}
	cpu.v[o.x] = cpu.v[o.y] - cpu.v[o.x]
}

type ShiftRightX struct {
	x uint16
}

func (o *ShiftRightX) Execute(cpu *Processor, _ *uint8) {
	cpu.v[CarryFlag] = cpu.v[o.x] & 0x1
	cpu.v[o.x] >>= 1
}

type ShiftLeftX struct {
	x uint16
}

func (o *ShiftLeftX) Execute(cpu *Processor, _ *uint8) {
	cpu.v[CarryFlag] = (cpu.v[o.x] & 0x80) >> 7
	cpu.v[o.x] <<= 1
}

type SetIToNNN struct {
	nnn uint16
}

func (o *SetIToNNN) Execute(cpu *Processor, _ *uint8) {
	cpu.i = o.nnn
}

type SetXToRandom struct {
	x  uint16
	nn uint16
}

func (o *SetXToRandom) Execute(cpu *Processor, _ *uint8) {
	randomByte := byte(rand.Uint32N(256))
	cpu.v[o.x] = randomByte & byte(o.nn)
}

type DrawSprite struct {
	x uint16
	y uint16
	n uint16
}

func (o *DrawSprite) Execute(cpu *Processor, info *uint8) {
	cpu.DrawSprite(cpu.v[o.x], cpu.v[o.y], byte(o.n))
	*info |= Redraw
}

type StepIfKeyDown struct {
	x uint16
}

func (o *StepIfKeyDown) Execute(cpu *Processor, _ *uint8) {
	key := cpu.v[o.x] & 0x0F
	if cpu.keyState[key].Load() {
		cpu.pc += 2
	}
}

type StepIfKeyUp struct {
	x uint16
}

func (o *StepIfKeyUp) Execute(cpu *Processor, _ *uint8) {
	key := cpu.v[o.x] & 0x0F
	if !cpu.keyState[key].Load() {
		cpu.pc += 2
	}
}

type SetXToDelay struct {
	x uint16
}

func (o *SetXToDelay) Execute(cpu *Processor, _ *uint8) {
	cpu.v[o.x] = cpu.delay
}

type PauseUntilKeyPressed struct {
	x uint16
}

func (o *PauseUntilKeyPressed) Execute(cpu *Processor, _ *uint8) {
	var keyPressed bool

	for i := range uint8(len(cpu.keyState)) {
		if cpu.keyState[i].Load() {
			keyPressed = true
			cpu.v[o.x] = i
			break
		}
	}

	if !keyPressed {
		cpu.pc -= 2 // Move the program counter back, replaying the last opcode
	}
}

type SetDelayToX struct {
	x uint16
}

func (o *SetDelayToX) Execute(cpu *Processor, _ *uint8) {
	cpu.delay = cpu.v[o.x]
}

type SetSoundToX struct {
	x uint16
}

func (o *SetSoundToX) Execute(cpu *Processor, _ *uint8) {
	cpu.sound = cpu.v[o.x]
}

type SetIToX struct {
	x uint16
}

func (o *SetIToX) Execute(cpu *Processor, _ *uint8) {
	cpu.i += uint16(cpu.v[o.x])
}

type SetIToSymbol struct {
	x uint16
}

func (o *SetIToSymbol) Execute(cpu *Processor, _ *uint8) {
	digit := uint16(cpu.v[o.x] & 0x0F)
	cpu.i = FontStartAddress + (digit * 5)
}

type BinaryCodedDecimal struct {
	x uint16
}

func (o *BinaryCodedDecimal) Execute(cpu *Processor, _ *uint8) {
	// Takes the number in register VX (which is one byte, so it can be any number from
	// 0 to 255) and converts it to three decimal digits, storing these digits in memory
	// at the address in the index register I. For example, if VX contains 156 (or 9C in
	// hexadecimal), it would put the number 1 at the address in I, 5 in address I + 1,
	// and 6 in address I + 2.

	// Double Dabble algorithm.
	//
	// Converts binary numbers to Binary-Coded Decimal (BCD) by repeatedly shifting
	// and adding 3 to nibbles that exceed 4, effectively performing a base conversion
	// in hardware. It starts with a binary input and an empty BCD register, iterating
	// for each bit, left-shifting the combined register, and injecting the next binary
	// bit, adding 3 to any BCD nibble >= 5 to handle carries, making it efficient for
	// digital displays.
	//
	// "Double": Each left shift effectively multiplies the BCD digits by 2.
	//
	// "Dabble": Adding 3 when a nibble hits 5 or more ensures that when it's shifted,
	//           it carries over correctly (e.g., 5 becomes 8, shift makes it 16, which
	//           is 10 in decimal, correctly carrying to the next place).
	//
	// The idea is that this implementation should be more efficient than integer division
	// and modulo operations.
	var bcd uint32

	// Fetch the value from register VX as a 32bit integer.
	val := uint32(cpu.v[o.x])

	// Iterate 8 times (once for each bit of the input byte)
	// Check each BCD nibble. If >= 5, add 3.
	for i := range 8 {
		// Ones (bits 0-3)
		if (bcd & 0x00F) >= 5 {
			bcd += 3
		}

		// Tens (bits 4-7)
		if (bcd & 0x0F0) >= 0x050 {
			bcd += 0x030
		}

		// Hundreds (bits 8-11)
		if (bcd & 0xF00) >= 0x500 {
			bcd += 0x300
		}

		// Shift BCD left by 1, and pull in the next bit from `val`
		bcd = (bcd << 1) | ((val >> (7 - i)) & 1)
	}

	cpu.memory[cpu.i] = byte((bcd >> 8) & 0xF)   // Hundreds
	cpu.memory[cpu.i+1] = byte((bcd >> 4) & 0xF) // Tens
	cpu.memory[cpu.i+2] = byte(bcd & 0xF)        // Ones
}

type SetRegistersToMemory struct {
	x uint16
}

func (o *SetRegistersToMemory) Execute(cpu *Processor, _ *uint8) {
	for i := uint16(0); i <= o.x; i++ {
		cpu.memory[cpu.i+i] = cpu.v[i]
	}
}

type SetMemoryToRegisters struct {
	x uint16
}

func (o *SetMemoryToRegisters) Execute(cpu *Processor, _ *uint8) {
	for i := uint16(0); i <= o.x; i++ {
		cpu.v[i] = cpu.memory[cpu.i+i]
	}
}

func Decode(opcode uint16) OpCode {
	// First nibble of the opcode is the operation kind.
	kind := (opcode & 0xF000) >> 12

	// Second nibble of the opcode is the X register location.
	x := (opcode & 0x0F00) >> 8

	// Third nibble of the opcode is the Y register location.
	y := (opcode & 0x00F0) >> 4

	// Fourth nibble of the opcode is the N value.
	n := opcode & 0x000F

	// Third and fourth nibbles of the opcode combine into the NN value.
	nn := opcode & 0x00FF

	// Second, third, and fourth nibbles of the opcode combine into the NNN value.
	nnn := opcode & 0x0FFF

	switch kind {
	case 0x0:
		switch opcode {
		case 0x00E0:
			return &ClearScreen{}
		case 0x00EE:
			return &SubroutineReturn{}
		default:
			panic("unknown 0x0 opcode")
		}
	case 0x1:
		return &JumpToLocation{nnn}
	case 0x2:
		return &SubroutineCall{nnn}
	case 0x3:
		return &StepIfXEqualsNN{x, nn}
	case 0x4:
		return &StepIfXNotEqualsNN{x, nn}
	case 0x5:
		return &StepIfXEqualsY{x, y}
	case 0x6:
		return &SetXToNN{x, nn}
	case 0x7:
		return &AddNNToX{x, nn}
	case 0x8:
		switch n {
		case 0x0:
			return &SetXToY{x, y}
		case 0x1:
			return &OrXY{x, y}
		case 0x2:
			return &AndXY{x, y}
		case 0x3:
			return &XOrXY{x, y}
		case 0x4:
			return &AddXY{x, y}
		case 0x5:
			return &SubtractYFromX{x, y}
		case 0x6:
			return &ShiftRightX{x}
		case 0x7:
			return &SubtractXFromY{x, y}
		case 0xE:
			return &ShiftLeftX{x}
		default:
			panic("unknown 0x8 opcode")
		}
	case 0x9:
		return &StepIfXNotEqualsY{x, y}
	case 0xA:
		return &SetIToNNN{nnn}
	case 0xB:
		return &JumpWithOffset{nnn}
	case 0xC:
		return &SetXToRandom{x, nn}
	case 0xD:
		return &DrawSprite{x, y, n}
	case 0xE:
		switch nn {
		case 0x9E:
			return &StepIfKeyDown{x}

		case 0xA1:
			return &StepIfKeyUp{x}
		default:
			panic("unknown 0xE opcode")
		}
	case 0xF:
		switch nn {
		case 0x07:
			return &SetXToDelay{x}
		case 0x0A:
			return &PauseUntilKeyPressed{x}
		case 0x15:
			return &SetDelayToX{x}
		case 0x18:
			return &SetSoundToX{x}
		case 0x1E:
			return &SetIToX{x}
		case 0x29:
			return &SetIToSymbol{x}
		case 0x33:
			return &BinaryCodedDecimal{x}
		case 0x55:
			return &SetRegistersToMemory{x}
		case 0x65:
			return &SetMemoryToRegisters{x}
		default:
			panic("unknown 0xF opcode")
		}
	default:
		panic("unknown opcode")
	}
}
