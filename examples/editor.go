package main

import (
	"github.com/rovaughn/termapp"
	"os"
)

type buffer struct {
	text []rune
	rows []int
}

func bufferFromString(s string) *buffer {
	b := new(buffer)
	textIndex := 0
	for _, r := range s {
		if r == '\n' {
			b.rows = append(b.rows, textIndex)
		} else {
			b.text = append(b.text, r)
			textIndex++
		}
	}
	return b
}

func (b *buffer) insert(i int, text string) {
}

// Optimization: this could do a binary search.
func (b *buffer) toRowCol(i int) (int, int) {
	if i < b.rows[0] {
		return 0, i
	}
	for row := 0; row < len(b.rows)-1; row++ {
		if b.rows[row] <= i && i < b.rows[row+1] {
			return 1 + row, i - b.rows[row]
		}
	}
	return len(b.rows), i - b.rows[len(b.rows)-1]
}

func (b *buffer) toIndex(row, col int) int {
	if row == 0 {
		return col
	}
	return b.rows[row-1] + col
}

func main() {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	buf := bufferFromString("Hello\nworld")
	cursor := 0

	t, err := termapp.NewTerminal(f, func(width, height int) *termapp.Screen {
		screen := termapp.NewScreen(width, height)

		for i, r := range buf.text {
			row, col := buf.toRowCol(i)
			if i == cursor {
				screen.SetCursor(col, row, true)
			}
			screen.PrintRune(col, row, termapp.Black, termapp.White, r)
			col++
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
			case termapp.KeyUp, 'k':
				row, col := buf.toRowCol(cursor)
				if row > 0 {
					cursor = buf.toIndex(row-1, col)
				}
			case termapp.KeyDown, 'j':
				row, col := buf.toRowCol(cursor)
				if row < len(buf.rows) {
					cursor = buf.toIndex(row+1, col)
				}
			case termapp.KeyRight, 'l':
				cursor++
			case termapp.KeyLeft, 'h':
				cursor--
			}
		case err := <-t.ErrCh:
			panic(err)
		}
		t.Redraw()
	}
}
