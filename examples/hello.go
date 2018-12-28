package main

import (
	"fmt"
	"github.com/rovaughn/termapp"
	"os"
	"time"
	"unicode"
)

func color(c int) termapp.Color {
	return termapp.Color{
		R: uint8(c >> 16),
		G: uint8(c >> 8),
		B: uint8(c >> 0),
	}
}

func main() {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	var letter termapp.Key
	tick := 0

	t, err := termapp.NewTerminal(f, func(width, height int) *termapp.Screen {
		screen := termapp.NewScreen(width, height)

		rainbow := []termapp.Color{
			color(0xff0000), color(0xffff00), color(0x00ff00),
			color(0x00ffff), color(0x0000ff), color(0xff00ff),
		}

		if letter < 128 && unicode.IsPrint(rune(letter)) {
			screen.PrintRune(width/2, height/2-1, termapp.Black, termapp.White, rune(letter))
		}
		for i, c := range "HELLO!" {
			screen.PrintRune(width/2+i, height/2, rainbow[(tick+i)%6], rainbow[(tick+i+3)%6], c)
		}
		for i, c := range "world" {
			screen.PrintRune(width/2+i, height/2+1, color(0x101010*i), termapp.White, c)
		}

		return screen
	})
	if err != nil {
		panic(err)
	}

	ticker := time.NewTicker(500 * time.Millisecond)

	for {
		select {
		case <-ticker.C:
			tick++
		case key := <-t.KeyCh:
			letter = key
		case err := <-t.ErrCh:
			fmt.Println(err)
		}
		t.Redraw()
	}
}
