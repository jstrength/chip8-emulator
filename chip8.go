package main

import (
    "io/ioutil"
    "math/rand"
    "log"
    "strings"

    "github.com/veandco/go-sdl2/sdl"
)

const (
    pixelSize = 10
    width = 64*pixelSize
    height = 32*pixelSize
)

var (
    opcode uint16
    memory [4096]uint8
    regsV [16]uint8 //CPU registers. 0-14 general purpose. 15 is 'carry flag'
    regI uint16 //index register
    pc uint16 //program counter
    gfx [64 * 32]uint8
    //timer registers that count at 60Hz
    delayTimer uint8
    soundTimer uint8
    stack [16]uint16
    sp uint16 //stack pointer
    key [16]uint8 //hex based keypad 0x0-0xF
    drawFlag bool

    chip8Fontset = [80]uint8{
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
        0xF0, 0x80, 0xF0, 0x80, 0x80,  // F
    }
    keyboardToKeypad = map[rune]uint8{
        '1':0x1, '2':0x2, '3':0x3, '4':0xC,
        'q':0x4, 'w':0x5, 'e':0x6, 'r':0xD,
        'a':0x7, 's':0x8, 'd':0x9, 'f':0xE,
        'z':0xA, 'x':0x0, 'c':0xB, 'v':0xF,
    }
    running bool
)

func loadGame(filename string) {
    log.Println("Loading", filename)
    buffer, err := ioutil.ReadFile(filename)
    if err != nil {
        log.Println("Unable to load", filename)
        panic(err)
    }

    for i, b := range buffer {
        memory[i + 512] = b
    }
}

func initialize() {
    pc = 0x200 // Program counter starts at 0x200
    opcode = 0
    regI = 0
    sp = 0
    drawFlag = false
    running = true

    gfx = [64 * 32]uint8{}
    stack = [16]uint16{}
    regsV = [16]uint8{}
    memory = [4096]uint8{}

    //load fontset
    for i := 0; i < 80; i++ {
        memory[i] = chip8Fontset[i]
    }

    delayTimer = 0
    soundTimer = 0
}

func emulateCycle() {
    //log.Printf("cycling... %X %X %d %d\n", pc, opcode, delayTimer, soundTimer)
    opcode = uint16(memory[pc]) << 8 | uint16(memory[pc + 1])

    switch opcode & 0xF000 {
    case 0x0000:
        switch opcode & 0x000F {
        case 0x0000:
            //clear screen
            gfx = [64 * 32]uint8{}
        case 0x000E:
            sp--
            pc = stack[sp]
            pc += 2
        default:
            log.Panicf("Unknown opcode [0x0000]: 0x%X\n", opcode)
        }
    case 0x1000:
        pc = opcode & 0x0FFF
    case 0x2000:
        stack[sp] = pc
        sp++
        pc = opcode & 0x0FFF
    case 0x3000:
        if regsV[(opcode & 0x0F00) >> 8] == uint8(opcode & 0x00FF) {
            pc += 4
        } else {
            pc += 2
        }
    case 0x4000:
        if regsV[(opcode & 0x0F00) >> 8] != uint8(opcode & 0x00FF) {
            pc += 4
        } else {
            pc += 2
        }
    case 0x6000:
        regsV[(opcode & 0x0F00) >> 8] = uint8(opcode & 0x00FF)
        pc += 2
    case 0x7000:
        regsV[(opcode & 0x0F00) >> 8] += uint8(opcode & 0x00FF)
        pc += 2
    case 0x8000:
        switch opcode & 0x000F {
        case 0x0000:
            regsV[(opcode & 0x0F00) >> 8] = regsV[(opcode & 0x00F0) >> 4]
            pc += 2
        case 0x0001:
            regsV[(opcode & 0x0F00) >> 8] |= regsV[(opcode & 0x00F0) >> 4]
            pc += 2
        case 0x0002:
            regsV[(opcode & 0x0F00) >> 8] &= regsV[(opcode & 0x00F0) >> 4]
            pc += 2
        case 0x0003:
            regsV[(opcode & 0x0F00) >> 8] ^= regsV[(opcode & 0x00F0) >> 4]
            pc += 2
        case 0x0004:
            if regsV[(opcode & 0x00F0) >> 4] > (0xFF - regsV[(opcode & 0x0F00) >> 8]) {
                regsV[0xF] = 1 //carry
            } else {
                regsV[0xF] = 0
            }
            pc += 2
        default:
            log.Panicf("Unknown opcode [0x8000]: 0x%X\n", opcode)
        }
    case 0x9000:
        if regsV[(opcode & 0x0F00) >> 8] != regsV[(opcode & 0x00F0) >> 4] {
            pc += 4
        } else {
            pc += 2
        }
    case 0xA000:
        regI = opcode & 0x0FFF
        pc += 2
    case 0xC000:
        randInt := uint8(rand.Intn(255) & int(opcode & 0x00FF))
        log.Println("Gen Random",randInt)
        regsV[(opcode & 0x0F00) >> 8] = randInt
        pc += 2
    case 0xD000:
        var (
            x uint16 = uint16(regsV[(opcode & 0x0F00) >> 8])
            y uint16 = uint16(regsV[(opcode & 0x00F0) >> 4])
            height uint16 = opcode & 0x000F
            pixel uint16
        )

        regsV[0xF] = 0
        for yLine := uint16(0); yLine < height; yLine++ {
            pixel = uint16(memory[regI + yLine])
            for xLine := uint16(0); xLine < 8; xLine++ {
                if (pixel & (0x80 >> xLine)) != 0 {
                    if gfx[x + xLine + ((y + yLine) * 64)] == 1 {
                        regsV[0xF] = 1
                    }
                    gfx[x + xLine + ((y + yLine) * 64)] ^= 1
                }
            }
        }

        drawFlag = true
        pc += 2
    case 0xE000:
        switch opcode & 0x00FF {
            // EX9E: Skips the next instruction
            // if the key stored in VX is pressed
        case 0x009E:
            if key[regsV[(opcode & 0x0F00) >> 8]] != 0 {
                pc += 4
            } else {
                pc += 2
            }
        case 0x00A1:
            if key[regsV[(opcode & 0x0F00) >> 8]] == 0 {
                pc += 4
            } else {
                pc += 2
            }
        default:
            log.Panicf("Unknown opcode [0xE000]: 0x%X\n", opcode)
        }
    case 0xF000:
        switch opcode & 0x00FF {
        case 0x000A:
            log.Println("Waiting for key input")
            regsV[(opcode & 0x0F00) >> 8] = getKeyPress()
            pc += 2
        case 0x0007:
            regsV[(opcode & 0x0F00) >> 8] = delayTimer
            pc += 2
        case 0x0015:
            delayTimer = regsV[(opcode & 0x0F00) >> 8]
            pc += 2
        case 0x001E:
            regI += uint16(regsV[(opcode & 0x0F00) >> 8])
            pc += 2
        case 0x0029:
            log.Println(regsV[(opcode & 0x0F00) >> 8])
            regI = uint16(regsV[(opcode & 0x0F00) >> 8]) * 0x4
            pc += 2
        case 0x0033:
            vX := (opcode & 0x0F00) >> 8
            memory[regI] = regsV[vX] / 100
            memory[regI + 1] = (regsV[vX] / 10) % 10
            memory[regI + 2] = (regsV[vX] % 100) % 10
            pc += 2
        case 0x0055:
            for i := uint16(0); i <= (opcode & 0x0F00) >> 8; i++ {
                memory[regI + i] = regsV[i]
            }
            pc += 2
        case 0x0065:
            for i := uint16(0); i <= (opcode & 0x0F00) >> 8; i++ {
                regsV[i] = memory[regI + i]
            }
            pc += 2
        default:
            log.Panicf("Unknown opcode [0xF000]: 0x%X\n", opcode)
        }

    default:
        log.Panicf("Unknown opcode: 0x%X\n", opcode)
    }

    if delayTimer > 0 {
        delayTimer--
    }

    if soundTimer > 0 {
        if soundTimer == 1 {
            log.Println("BEEP")
        }
        soundTimer--
    }
}

func drawGraphics(window *sdl.Window) {
	surface, err := window.GetSurface()
	if err != nil {
		panic(err)
	}
	surface.FillRect(nil, 0)


    for i, v := range gfx {
        rect := sdl.Rect{int32((i%64)*pixelSize), int32((i/64)*pixelSize), 10, 10}
        if v == 1 {
            surface.FillRect(&rect, 0xffff0000)
        } else {
            surface.FillRect(&rect, 0x00000000)
        }
    }
	window.UpdateSurface()
}

func getKeyPress() uint8 {
    for {
        for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
            switch t := event.(type) {
            case *sdl.KeyboardEvent:
                if strings.ContainsRune("1234qwerasdfzxcv", rune(t.Keysym.Sym)) && t.State == 0 {
                    k := keyboardToKeypad[rune(t.Keysym.Sym)]
                    log.Println("GetKeyPress",k)
                    return k
                }
            case *sdl.QuitEvent:
                log.Println("Quit")
                running = false
                return 0
            }
            sdl.Delay(17)
        }
    }
}

func listenKeyboard() {
    for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
        switch t := event.(type) {
        case *sdl.KeyboardEvent:
            if strings.ContainsRune("1234qwerasdfzxcv", rune(t.Keysym.Sym)) {
                k := keyboardToKeypad[rune(t.Keysym.Sym)]
                key[k] = t.State
                log.Println("ListenKeyboard",k)
            }
        case *sdl.QuitEvent:
            log.Println("Quit")
            running = false
        }
    }
}

func main() {
    log.Println("Chip8 Emulator")

    if err := sdl.Init(sdl.INIT_EVERYTHING); err != nil {
		panic(err)
	}
	defer sdl.Quit()

    window, err := sdl.CreateWindow("test", sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED,
		width, height, sdl.WINDOW_SHOWN)
	if err != nil {
		panic(err)
	}
	defer window.Destroy()

    initialize()
    loadGame("roms/connect4.rom")

    for running {
        emulateCycle()
        if drawFlag {
            drawGraphics(window)
            drawFlag = false
        }
        listenKeyboard()
        sdl.Delay(17)
    }
}
