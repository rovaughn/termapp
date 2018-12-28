package main

import (
	"github.com/rovaughn/termapp"
	"math/cmplx"
	"os"
)

func main() {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	const iters = 32
	const zoomFactor = 1.0 - (1.0 / 16.0)
	const translateInc = 1.0 / 8.0
	minX, maxX := -2.1, 0.6
	minY, maxY := -1.2, 1.2
	palette := ".-+*%#$@ "
	totalZoom := 1.0

	t, err := termapp.NewTerminal(f, func(width, height int) *termapp.Screen {
		screen := termapp.NewScreen(width, height)
		incX := (maxX - minX) / float64(width)
		incY := (maxY - minY) / float64(height)
		y := minY
		for row := 0; row < height; row++ {
			x := minX
			for col := 0; col < width; col++ {
				z := 0i
				i := 0
				for i < iters {
					z = z*z + complex(x, y)
					if cmplx.Abs(z) > 2 {
						break
					}
					i++
				}
				screen.PrintRune(col, row, termapp.Black, termapp.White, rune(palette[int(i*8/iters)]))
				x += incX
			}
			y += incY
		}
		return screen
	})
	if err != nil {
		panic(err)
	}
	defer t.Close()

	for {
		select {
		case key := <-t.KeyCh:
			switch key {
			case 'h':
				minX -= translateInc * totalZoom
				maxX -= translateInc * totalZoom
			case 'l':
				minX += translateInc * totalZoom
				maxX += translateInc * totalZoom
			case 'j':
				minY += translateInc * totalZoom
				maxY += translateInc * totalZoom
			case 'k':
				minY -= translateInc * totalZoom
				maxY -= translateInc * totalZoom
			case 'q':
				lengthX, lengthY := maxX-minX, maxY-minY
				centerX, centerY := minX+lengthX/2, minY+lengthY/2
				newLengthX, newLengthY := lengthX*zoomFactor, lengthY*zoomFactor
				minX, minY = centerX-newLengthX/2, centerY-newLengthY/2
				totalZoom *= zoomFactor
			case 'a':
				lengthX, lengthY := maxX-minX, maxY-minY
				centerX, centerY := minX+lengthX/2, minY+lengthY/2
				newLengthX, newLengthY := lengthX/zoomFactor, lengthY/zoomFactor
				minX, minY = centerX-newLengthX/2, centerY-newLengthY/2
				totalZoom /= zoomFactor
			}
		case err := <-t.ErrCh:
			panic(err)
		}
		t.Redraw()
	}
}
