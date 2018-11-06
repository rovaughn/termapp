package main

import (
	"fmt"
	"math/rand"
	"os"
	"sync/atomic"
	"time"
)

func main() {
	// pf, err := os.Create("cpuprofile")
	// if err != nil {
	// 	panic(err)
	// }
	// defer pf.Close()
	// pprof.StartCPUProfile(pf)
	// defer pprof.StopCPUProfile()

	f, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	var tick uint64
	var elapsedSeconds uint64
	var elapsedTicks uint64

	go func() {
		for range time.Tick(time.Second) {
			atomic.AddUint64(&elapsedSeconds, 1)
			atomic.StoreUint64(&elapsedTicks, atomic.LoadUint64(&tick))
		}
	}()

	t, err := NewTerminal(f, func(width, height int) *Screen {
		screen := NewScreen(width, height)

		es := atomic.LoadUint64(&elapsedSeconds)
		et := atomic.LoadUint64(&elapsedTicks)

		screen.Print(0, 0, Black, White, fmt.Sprintf("%.1f", float64(et)/float64(es)))

		for x := 2; x < width-2; x++ {
			for y := 2; y < height-2; y++ {
				if rand.Uint32()&1 == 0 {
					screen.Print(
						x, y,
						Color{uint8(rand.Uint32()), uint8(rand.Uint32()), uint8(rand.Uint32())},
						Color{uint8(rand.Uint32()), uint8(rand.Uint32()), uint8(rand.Uint32())},
						"x",
					)
				}
			}
		}

		return screen
	})
	if err != nil {
		panic(err)
	}

	for atomic.LoadUint64(&elapsedSeconds) < 3 {
		if err := t.Redraw(); err != nil {
			panic(err)
		}
		atomic.AddUint64(&tick, 1)
	}
}
