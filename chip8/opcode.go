/*
 * Copyright 2026 Joshua Jones <joshua.jones.software@gmail.com>
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      www.apache.org
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package chip8

import (
	"math/rand/v2"
)

func clearScreen(info *uint8) {
	for i := range cpu.display {
		cpu.display[i] = 0
	}
	*info |= Redraw
}

func callSubroutine(nnn uint16) {
	if int(cpu.sp) >= len(cpu.stack) {
		panic("stack overflow")
	}
	cpu.stack[cpu.sp] = cpu.pc
	cpu.sp++
	cpu.pc = nnn
}

func returnFromSubroutine() {
	if cpu.sp == 0 {
		panic("stack underflow")
	}
	cpu.sp--
	cpu.pc = cpu.stack[cpu.sp]
}

func jumpToLocation(nnn uint16) {
	cpu.pc = nnn
}

func jumpWithOffset(nnn uint16) {
	cpu.pc = nnn + uint16(cpu.v[0x0])
}

func stepIfXEqualsNN(x, nn uint16) {
	if cpu.v[x] == byte(nn) {
		cpu.pc += 2
	}
}

func stepIfXNotEqualsNN(x, nn uint16) {
	if cpu.v[x] != byte(nn) {
		cpu.pc += 2
	}
}

func stepIfXEqualsY(x, y uint16) {
	if cpu.v[x] == cpu.v[y] {
		cpu.pc += 2
	}
}

func stepIfXNotEqualsY(x, y uint16) {
	if cpu.v[x] != cpu.v[y] {
		cpu.pc += 2
	}
}

func setXToNN(x, nn uint16) {
	cpu.v[x] = byte(nn)
}

func addNNToX(x, nn uint16) {
	cpu.v[x] += byte(nn)
}

func setXToY(x, y uint16) {
	cpu.v[x] = cpu.v[y]
}

func orXY(x, y uint16) {
	// This operation traditionally resets the carry flag.
	cpu.v[CarryFlag] = 0
	cpu.v[x] |= cpu.v[y]
}

func andXY(x, y uint16) {
	// This operation traditionally resets the carry flag.
	cpu.v[CarryFlag] = 0
	cpu.v[x] &= cpu.v[y]
}

func xorXY(x, y uint16) {
	// This operation traditionally resets the carry flag.
	cpu.v[CarryFlag] = 0
	cpu.v[x] ^= cpu.v[y]
}

func addXY(x, y uint16) {
	sum := uint16(cpu.v[x]) + uint16(cpu.v[y])
	// This operation traditionally resets the carry flag.
	cpu.v[CarryFlag] = 0
	if sum > 255 {
		cpu.v[CarryFlag] = 1
	}
	cpu.v[x] = byte(sum & 0xFF)
}

func subtractYFromX(x, y uint16) {
	cpu.v[CarryFlag] = 0
	if cpu.v[x] >= cpu.v[y] {
		cpu.v[CarryFlag] = 1
	}
	cpu.v[x] -= cpu.v[y]
}

func subtractXFromY(x, y uint16) {
	cpu.v[CarryFlag] = 0
	if cpu.v[y] >= cpu.v[x] {
		cpu.v[CarryFlag] = 1
	}
	cpu.v[x] = cpu.v[y] - cpu.v[x]
}

func shiftRightX(x uint16) {
	cpu.v[CarryFlag] = cpu.v[x] & 0x1
	cpu.v[x] >>= 1
}

func shiftLeftX(x uint16) {
	cpu.v[CarryFlag] = (cpu.v[x] & 0x80) >> 7
	cpu.v[x] <<= 1
}

func setIToNNN(nnn uint16) {
	cpu.i = nnn
}

func setXToRandom(x, nn uint16) {
	randomByte := byte(rand.Uint32N(256))
	cpu.v[x] = randomByte & byte(nn)
}

func drawSprite(x, y, n uint16, info *uint8) {
	DrawSprite(cpu.v[x], cpu.v[y], byte(n))
	*info |= Redraw
}

func stepIfKeyDown(x uint16) {
	key := cpu.v[x] & 0x0F
	if cpu.keyState[key].Load() {
		cpu.pc += 2
	}
}

func stepIfKeyUp(x uint16) {
	key := cpu.v[x] & 0x0F
	if !cpu.keyState[key].Load() {
		cpu.pc += 2
	}
}

func setXToDelay(x uint16) {
	cpu.v[x] = cpu.delay
}

func pauseUntilKeyPressed(x uint16) {
	var keyPressed bool

	for i := range uint8(len(cpu.keyState)) {
		if cpu.keyState[i].Load() {
			keyPressed = true
			cpu.v[x] = i
			break
		}
	}

	if !keyPressed {
		cpu.pc -= 2 // Move the program counter back, replaying the last opcode
	}
}

func setDelayToX(x uint16) {
	cpu.delay = cpu.v[x]
}

func setSoundToX(x uint16) {
	cpu.sound = cpu.v[x]
}

func setIToX(x uint16) {
	cpu.i += uint16(cpu.v[x])
}

func setIToSymbol(x uint16) {
	digit := uint16(cpu.v[x] & 0x0F)
	cpu.i = FontStartAddress + (digit * 5)
}

func binaryCodedDecimal(x uint16) {
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
	val := uint32(cpu.v[x])

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

func setRegistersToMemory(x uint16) {
	for i := uint16(0); i <= x; i++ {
		cpu.memory[cpu.i+i] = cpu.v[i]
	}
}

func setMemoryToRegisters(x uint16) {
	for i := uint16(0); i <= x; i++ {
		cpu.v[i] = cpu.memory[cpu.i+i]
	}
}

func execute(opcode uint16, info *uint8) {
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
			clearScreen(info)
		case 0x00EE:
			returnFromSubroutine()
		default:
			panic("unknown 0x0 opcode")
		}
	case 0x1:
		jumpToLocation(nnn)
	case 0x2:
		callSubroutine(nnn)
	case 0x3:
		stepIfXEqualsNN(x, nn)
	case 0x4:
		stepIfXNotEqualsNN(x, nn)
	case 0x5:
		stepIfXEqualsY(x, y)
	case 0x6:
		setXToNN(x, nn)
	case 0x7:
		addNNToX(x, nn)
	case 0x8:
		switch n {
		case 0x0:
			setXToY(x, y)
		case 0x1:
			orXY(x, y)
		case 0x2:
			andXY(x, y)
		case 0x3:
			xorXY(x, y)
		case 0x4:
			addXY(x, y)
		case 0x5:
			subtractYFromX(x, y)
		case 0x6:
			shiftRightX(x)
		case 0x7:
			subtractXFromY(x, y)
		case 0xE:
			shiftLeftX(x)
		default:
			panic("unknown 0x8 opcode")
		}
	case 0x9:
		stepIfXNotEqualsY(x, y)
	case 0xA:
		setIToNNN(nnn)
	case 0xB:
		jumpWithOffset(nnn)
	case 0xC:
		setXToRandom(x, nn)
	case 0xD:
		drawSprite(x, y, n, info)
	case 0xE:
		switch nn {
		case 0x9E:
			stepIfKeyDown(x)
		case 0xA1:
			stepIfKeyUp(x)
		default:
			panic("unknown 0xE opcode")
		}
	case 0xF:
		switch nn {
		case 0x07:
			setXToDelay(x)
		case 0x0A:
			pauseUntilKeyPressed(x)
		case 0x15:
			setDelayToX(x)
		case 0x18:
			setSoundToX(x)
		case 0x1E:
			setIToX(x)
		case 0x29:
			setIToSymbol(x)
		case 0x33:
			binaryCodedDecimal(x)
		case 0x55:
			setRegistersToMemory(x)
		case 0x65:
			setMemoryToRegisters(x)
		default:
			panic("unknown 0xF opcode")
		}
	default:
		panic("unknown opcode")
	}
}
