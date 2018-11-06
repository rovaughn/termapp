package main

import (
	"golang.org/x/crypto/ssh/terminal"
	"os"
	"strconv"
	"unicode/utf8"
)

// Opportunities for optimization:
// - We don't need to set the foreground color of the cursor if we're just
//   changing the background color of a blank cell.
// - If a cell is blank we can ignore any foreground color change.
// - We might be able to make color specifications shorter if it matches a
//   "standard" color.
// - When printing a character at the right edge of the screen, update the
//   cursor correctly with wrapping.

// Bugs:
// - After a wide character is printed, cursor position needs to be updated
//   correctly.

// CSI is "\x1b["

type Color struct {
	R, G, B uint8
}

// Will these "simple colors" come in handy?
// const (
// 	Black = iota
// 	Red
// 	Green
// 	Yellow
// 	Blue
// 	Magenta
// 	Cyan
// 	White
// )

//type Flags uint8
//
//const (
//	Bold = Flags(iota)
//	Dim
//	Underline
//	Inverse
//	Invisible
//	Strikethrough
//)

type Style struct {
	fore Color
	back Color
	//flags Flags
}

type Cell struct {
	Style
	text rune
}

type Cursor struct {
	Style
	x, y    int
	visible bool
}

type Screen struct {
	width, height int
	cells         []Cell
	cursor        Cursor
}

func NewScreen(width, height int) *Screen {
	return &Screen{
		width:  width,
		height: height,
		cells:  make([]Cell, width*height),
	}
}

var Black = Color{0x00, 0x00, 0x00}
var White = Color{0xff, 0xff, 0xff}

func (s *Screen) Print(x, y int, back, fore Color, text string) {
	start := y*s.width + x
	for i, r := range text {
		s.cells[start+i] = Cell{
			Style: Style{
				back: back,
				fore: fore,
			},
			text: r,
		}
	}
}

type Terminal struct {
	f      *os.File
	render RenderFunc
	buf    []byte
	Screen
}

func (t *Terminal) flush() error {
	n, err := t.f.Write(t.buf)
	if n == len(t.buf) {
		t.buf = t.buf[:0]
	} else {
		copy(t.buf, t.buf[n:])
		t.buf = t.buf[:len(t.buf)-n]
	}
	return err
}

type RenderFunc func(width, height int) *Screen

func NewTerminal(f *os.File, render RenderFunc) (*Terminal, error) {
	width, height, err := terminal.GetSize(int(f.Fd()))
	if err != nil {
		return nil, err
	}

	t := &Terminal{
		f:      f,
		render: render,
		Screen: Screen{
			width:  width,
			height: height,
			cells:  make([]Cell, width*height),
		},
	}

	// Clear the screen, so that we are in a known state.
	t.clear()
	t.redraw()

	if err := t.flush(); err != nil {
		return nil, err
	}

	return t, nil
}

func (t *Terminal) clear() {
	t.buf = append(t.buf, []byte("\x1b[H\x1b[2J\x1b[0m\x1b[?25l")...)
}

func (t *Terminal) moveCursor(x, y int) {
	if t.cursor.x == x && t.cursor.y == y {
		return
	}

	t.buf = append(t.buf, []byte("\x1b[")...)
	t.buf = strconv.AppendInt(t.buf, int64(y)+1, 10)
	t.buf = append(t.buf, ';')
	t.buf = strconv.AppendInt(t.buf, int64(x)+1, 10)
	t.buf = append(t.buf, 'H')

	t.cursor.x = x
	t.cursor.y = y
}

var numTable = func() (table []string) {
	table = make([]string, 256)
	for i := 0; i < 256; i++ {
		table[i] = strconv.Itoa(i)
	}
	return
}()

func (t *Terminal) setCursorStyle(s Style) {
	// SGR is short for Select Graphic Rendition
	//var sgrs []int
	sgrs := make([]int, 0, 10)

	if s.fore != t.cursor.fore {
		sgrs = append(sgrs, 38, 2, int(s.fore.R), int(s.fore.G), int(s.fore.B))
		t.cursor.fore = s.fore
	}

	if s.back != t.cursor.back {
		sgrs = append(sgrs, 48, 2, int(s.back.R), int(s.back.G), int(s.back.B))
		t.cursor.back = s.back
	}

	if len(sgrs) > 0 {
		t.buf = append(t.buf, []byte("\x1b[")...)
		//t.buf = strconv.AppendInt(t.buf, sgrs[0], 10)
		t.buf = append(t.buf, numTable[sgrs[0]]...)
		for _, sgr := range sgrs[1:] {
			t.buf = append(t.buf, ';')
			//t.buf = strconv.AppendInt(t.buf, sgr, 10)
			t.buf = append(t.buf, numTable[sgr]...)
		}
		t.buf = append(t.buf, 'm')
	}
}

func (t *Terminal) setCursorVisibility(visible bool) {
	if visible && !t.cursor.visible {
		t.buf = append(t.buf, []byte("\x1b[?25l")...)
	} else if !visible && t.cursor.visible {
		t.buf = append(t.buf, []byte("\x1b[?25h")...)
	}
}

func (t *Terminal) redraw() {
	newScreen := t.render(t.width, t.height)

	t.setCursorVisibility(newScreen.cursor.visible)

	p := make([]byte, 4)

	for y := 0; y < t.height; y++ {
		for x := 0; x < t.width; x++ {
			i := y*t.width + x

			a := t.cells[i]
			b := newScreen.cells[i]

			if a != b {
				t.moveCursor(x, y)
				t.setCursorStyle(b.Style)
				text := b.text
				if text == 0 {
					text = ' '
				}
				n := utf8.EncodeRune(p, text)
				t.buf = append(t.buf, p[:n]...)
				t.cells[i].text = text
				t.cursor.x++
			}
		}
	}

	t.moveCursor(newScreen.cursor.x, newScreen.cursor.y)
}

func (t *Terminal) Redraw() error {
	t.redraw()
	return t.flush()
}
