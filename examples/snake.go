package main

import (
	"fmt"
	"github.com/rovaughn/termapp"
	"math/rand"
	"os"
	"time"
)

func main() {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	const maxSegmentCount = 10

	type direction int
	const (
		north = direction(iota)
		south
		east
		west
	)

	type winState int
	const (
		playing = winState(iota)
		win
		lose
	)

	type point struct{ x, y int }

	var state struct {
		state    winState
		segments []point
		food     point
		dir      direction
	}

	state.food = point{5, 5}
	state.segments = make([]point, 0, maxSegmentCount)
	state.segments = append(state.segments, point{10, 10})

	t, err := termapp.NewTerminal(f, func(width, height int) *termapp.Screen {
		screen := termapp.NewScreen(width, height)

		screen.Print(0, 0, termapp.Black, termapp.White, fmt.Sprintf("%d/%d food gathered", len(state.segments)-1, maxSegmentCount-1))

		if state.state == lose {
			screen.Print(width/2, height/2, termapp.Black, termapp.White, "You lose!")
		} else if state.state == win {
			screen.Print(width/2, height/2, termapp.Black, termapp.White, "You win!")
		} else {
			for i, segment := range state.segments {
				color := uint8(255 - 10*i)
				if color < 128 {
					color = 128
				}
				screen.PrintRune(1+segment.x, 1+segment.y, termapp.Color{0x00, color, 0x00}, termapp.White, ' ')
			}
			screen.PrintRune(1+state.food.x, 1+state.food.y, termapp.Color{0xff, 0x00, 0x00}, termapp.White, ' ')
		}

		return screen
	})
	if err != nil {
		panic(err)
	}
	defer t.Close()

	move := func() {
		oldHead := state.segments[0]

		var newHead point
		switch state.dir {
		case north:
			newHead = point{oldHead.x, oldHead.y - 1}
		case south:
			newHead = point{oldHead.x, oldHead.y + 1}
		case east:
			newHead = point{oldHead.x + 1, oldHead.y}
		case west:
			newHead = point{oldHead.x - 1, oldHead.y}
		}

		if newHead == state.food {
			state.segments = append(state.segments, point{})
			state.food = point{
				rand.Intn(10),
				rand.Intn(10),
			}
		}

		copy(state.segments[1:], state.segments)
		state.segments[0] = newHead
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			move()
		case key := <-t.KeyCh:
			switch key {
			case termapp.KeyUp:
				state.dir = north
			case termapp.KeyDown:
				state.dir = south
			case termapp.KeyRight:
				state.dir = east
			case termapp.KeyLeft:
				state.dir = west
			}
		case err := <-t.ErrCh:
			panic(err)
		}
		t.Redraw()
	}
}
