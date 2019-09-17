package main

import (
  "fmt"
  "os"
  "io/ioutil"
  "math/bits"
  "math/rand"
  sdl "github.com/veandco/go-sdl2/sdl"
)

type chip8 struct {
  memory [4096]uint8 // 4k of memory
  reg [16]uint8 // cpu registers
  index_reg uint16 // index register
  pc uint16 // program counter
  sp uint8 // stack pointer
  stack [16]uint16
  display [32][64]bool // 64x32 pixel black and white display
  keypad uint16 // one input for eeach hex digit
  delay_timer uint8
  sound_timer uint8
}

var font_set = [...]uint8 {
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

func initSystem(rom_name string) chip8 {
  ret := chip8{}

  // load fonts and rom into memory
  for i, b := range font_set {
    ret.memory[i] = b
  }

  // program memory starts at 0x200
  ret.pc = 0x200
  rom_bytes, _:= ioutil.ReadFile(rom_name)
  for i, b := range rom_bytes {
    ret.memory[0x200 + i] = b
  }
  return ret
}

func updateDisplay(canvas *sdl.Renderer, chip chip8, scale int32) {
  // 0, 0 is top left
  for y, row := range chip.display {
    for x, pixel := range row {
      p := uint8(sdl.Btoi(pixel))
      canvas.SetDrawColor(p * 255, p * 255, p * 255, 255)
      canvas.FillRect(&sdl.Rect{X: int32(x) * scale, Y: int32(y) * scale, W: scale, H: scale})
    }
  }
  canvas.Present()
}

func updateKeypad(chip *chip8) {
  chip.keypad = 0
  ks := sdl.GetKeyboardState()
  for i := 0; i <= 0xF; i++ {
    s := sdl.GetScancodeFromName(fmt.Sprintf("%x", i))
    chip.keypad |= uint16(ks[s]) << i
  }
}

func step(chip *chip8, inc_timer bool) bool {
  opcode := uint16(chip.memory[chip.pc]) << 8 | uint16(chip.memory[chip.pc + 1])
  chip.pc += 2

  X := (opcode & 0x0F00) >> 8
  Y := (opcode & 0x00F0) >> 4
  NN := uint8(opcode & 0x00FF)
  NNN := opcode & 0x0FFF

  switch opcode >> 12 {
  case 0x0:
    if opcode == 0xE0 { // clear display
      for y, row := range chip.display {
        for x, _ := range row {
          chip.display[y][x] = false
        }
      }
    } else if opcode == 0xEE { // return from subroutine
      chip.pc = chip.stack[chip.sp]
      chip.sp--
    }
  case 0x1:
    chip.pc = NNN
  case 0x2 : // call the subroutine at NNN
    chip.sp++
    chip.stack[chip.sp] = chip.pc
    chip.pc = NNN
  case 0x3:
    if chip.reg[X] == NN {
      chip.pc += 2
    }
  case 0x4:
    if chip.reg[X] != NN {
      chip.pc += 2
    }
  case 0x5:
    if chip.reg[X] == chip.reg[Y] {
      chip.pc += 2
    }
  case 0x6:
    chip.reg[X] = NN
  case 0x7:
    chip.reg[X] += NN
  case 0x8:
    switch opcode & 0xF {
    case 0x0:
      chip.reg[X] = chip.reg[Y]
    case 0x1:
      chip.reg[X] |= chip.reg[Y]
    case 0x2:
      chip.reg[X] &= chip.reg[Y]
    case 0x3:
      chip.reg[X] ^= chip.reg[Y]
    case 0x4:
      chip.reg[0xF] = uint8(sdl.Btoi(uint16(chip.reg[X]) + uint16(chip.reg[Y]) > 0xFF))
      chip.reg[X] += chip.reg[Y]
    case 0x5:
      chip.reg[0xF] = uint8(sdl.Btoi(chip.reg[X] > chip.reg[Y]))
      chip.reg[X] -= chip.reg[Y]
    case 0x6:
      chip.reg[0xF] = chip.reg[X] & 0x1
      chip.reg[X] >>= 1
    case 0x7:
      chip.reg[0xF] = uint8(sdl.Btoi(chip.reg[Y] > chip.reg[X]))
      chip.reg[X] = chip.reg[Y] - chip.reg[X]
    case 0xE:
      chip.reg[0xF] = chip.reg[X] >> 7
      chip.reg[X] <<= 1
    }
  case 0x9:
    if chip.reg[X] != chip.reg[Y] {
      chip.pc += 2
    }
  case 0xA:
    chip.index_reg = NNN
  case 0xB:
    chip.pc = uint16(chip.reg[0]) + NNN
  case 0xC:
    chip.reg[X] = uint8(rand.Uint32()) & NN
  case 0xD:
    // draw a sprite
    chip.reg[0xF] = 0
    x_min, y_min := int(chip.reg[X]), int(chip.reg[Y])
    for y := y_min; y < y_min + int(opcode & 0xF); y++ {
      s := chip.memory[int(chip.index_reg) + (y - y_min)]
      for i := 0; i < 8; i++ {
        pixel := &chip.display[y % 32][(x_min + i) % 64]
        pixel_prev := *pixel
        *pixel = *pixel != (((s >> uint8(7 - i)) & 0x1) != 0)
        chip.reg[0xF] |= uint8(sdl.Btoi(!(*pixel) && pixel_prev))
      }
    }
  case 0xE:
    key := (chip.keypad >> uint16(chip.reg[X])) & 0x1
    if (opcode & 0xFF == 0x9E && key != 0) || (opcode & 0xFF == 0xA1 && key != 0) {
      chip.pc += 2
    }
  case 0xF:
    switch opcode & 0xFF {
    case 0x07:
      chip.reg[X] = chip.delay_timer
    case 0x0A:
      chip.pc -= 2
      if bits.OnesCount16(chip.keypad) > 0 {
        chip.reg[X] = uint8(bits.Len16(chip.keypad) - 1)
        chip.pc += 2
      }
    case 0x15:
      chip.delay_timer = chip.reg[X]
    case 0x18:
      chip.sound_timer = chip.reg[X]
    case 0x1E:
      chip.index_reg += uint16(chip.reg[X])
    case 0x29:
      chip.index_reg = uint16(chip.reg[X]) * uint16(5)
    case 0x33:
      chip.memory[chip.index_reg] = chip.reg[X] / 100
      chip.memory[chip.index_reg + 1] = (chip.reg[X] / 10) % 10
      chip.memory[chip.index_reg + 2] = chip.reg[X] % 10
    case 0x55:
      var i uint16
      for i = 0; i <= X; i++ {
        chip.memory[chip.index_reg + i] = chip.reg[i]
      }
    case 0x65:
      var i uint16
      for i = 0; i <= X; i++ {
        chip.reg[i] = chip.memory[chip.index_reg + i]
      }
    }
  default:
    fmt.Printf("NOT IMPLEMENTED 0x%x\n", opcode)
  }

  if inc_timer {
    if chip.sound_timer > 0 {
      chip.sound_timer--
    }
    if chip.delay_timer > 0 {
      chip.delay_timer--
    }
  }

  return opcode >> 12 == 0xD
}

func main() {
  const cpu_freq uint32 = 500
  const display_scale int32 = 10

  // init graphics
  sdl.Init(sdl.INIT_EVERYTHING)
  window, canvas, _ := sdl.CreateWindowAndRenderer(64*display_scale, 32*display_scale, sdl.WINDOW_SHOWN)
  window.SetTitle("chip8")

  defer sdl.Quit()
  defer window.Destroy()
  defer canvas.Destroy()

  rom := "roms/maze.ch8"
  if len(os.Args) > 1 {
    rom = os.Args[1]
  }
  fmt.Println("loading rom ", rom)
  chip := initSystem(rom)

  // main emulation loop
  inc_timer := cpu_freq / 60
  for {
    st := sdl.GetTicks()

    sdl.PollEvent()
    updateKeypad(&chip)

    gfx_updated := step(&chip, inc_timer == 0) // one cpu cycle
    if gfx_updated {
      updateDisplay(canvas, chip, display_scale)
    }

    if chip.sound_timer == 0 {
      // TODO: play sound
    }

    if sdl.GetTicks() - st < 1000 / cpu_freq {
      sdl.Delay((1000 / cpu_freq) - (sdl.GetTicks() - st))
    }

    if inc_timer == 0 {
      inc_timer = cpu_freq / 60
    } else {
      inc_timer--
    }
  }
}

