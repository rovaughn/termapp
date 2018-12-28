package termapp

import (
	"bufio"
	"fmt"
	"golang.org/x/crypto/ssh/terminal"
	"io"
	"os"
	"strconv"
	"syscall"
	"unicode/utf8"
	"unsafe"
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

type Style struct {
	fore Color
	back Color
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

func (s *Screen) SetCursor(x, y int, visible bool) {
	s.cursor.x = x
	s.cursor.y = y
	s.cursor.visible = visible
}

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

func (s *Screen) PrintRune(x, y int, back, fore Color, text rune) {
	s.cells[y*s.width+x] = Cell{
		Style: Style{
			back: back,
			fore: fore,
		},
		text: text,
	}
}

type Terminal struct {
	f            *os.File
	Logf         io.Writer
	render       RenderFunc
	buf          []byte
	savedTermios syscall.Termios
	ErrCh        chan error
	KeyCh        chan Key
	Screen
}

func (t *Terminal) flush() error {
	if t.Logf != nil {
		t.Logf.Write(t.buf)
	}

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

	var termios syscall.Termios
	if _, _, err := syscall.Syscall(
		syscall.SYS_IOCTL,
		f.Fd(),
		syscall.TCGETS,
		uintptr(unsafe.Pointer(&termios)),
	); err != 0 {
		return nil, fmt.Errorf("tcgetattr: %s", err)
	}

	t := &Terminal{
		f:            f,
		savedTermios: termios,
		render:       render,
		Screen: Screen{
			width:  width,
			height: height,
			cells:  make([]Cell, width*height),
		},
		KeyCh: make(chan Key),
		ErrCh: make(chan error),
	}

	termios.Lflag ^= syscall.ICANON | syscall.ECHO
	termios.Cc[syscall.VMIN] = 1
	termios.Cc[syscall.VTIME] = 0

	if _, _, err := syscall.Syscall(
		syscall.SYS_IOCTL,
		f.Fd(),
		syscall.TCSETS,
		uintptr(unsafe.Pointer(&termios)),
	); err != 0 {
		return nil, fmt.Errorf("tcsetattr: %s", err)
	}

	go t.readKeys()

	// Clear the screen, so that we are in a known state.
	t.clear(width, height)
	t.redraw()

	if err := t.flush(); err != nil {
		return nil, err
	}

	return t, nil
}

func (t *Terminal) Close() error {
	if _, _, err := syscall.Syscall(
		syscall.SYS_IOCTL,
		t.f.Fd(),
		syscall.TCSETS,
		uintptr(unsafe.Pointer(&t.savedTermios)),
	); err != 0 {
		return fmt.Errorf("tcsetattr: %s", err)
	}

	return nil
}

type Key uint16

// It might make sense to make every special key 128 + its escape character,
// e.g. KeyDown = 128 + 'B' = 128 + 66 = 194
const (
	KeyDel = 128 + iota
	KeyEnd
	KeyUp
	KeyDown
	KeyRight
	KeyLeft
	KeyHome
)

func (t *Terminal) readKeys() {
	keyMap := map[byte]Key{
		'3': KeyDel,
		'4': KeyEnd,
		'A': KeyUp,
		'B': KeyDown,
		'C': KeyRight,
		'D': KeyLeft,
		'H': KeyHome,
	}
	r := bufio.NewReader(t.f)

	for {
		c, err := r.ReadByte()
		if err != nil {
			t.ErrCh <- err
			return
		}

		if c == '\x1b' {
			c2, err := r.ReadByte()
			if err != nil {
				t.ErrCh <- err
				return
			}

			if c2 == '[' {
				c3, err := r.ReadByte()
				if err != nil {
					t.ErrCh <- err
					return
				}

				key, ok := keyMap[c3]
				if !ok {
					t.ErrCh <- fmt.Errorf("Unknown escape key %q", c3)
					return
				}

				t.KeyCh <- key
			} else {
				t.KeyCh <- Key(c)
				t.KeyCh <- Key(c2)
			}
		} else {
			t.KeyCh <- Key(c)
		}
	}
}

func (t *Terminal) clear(width, height int) {
	t.setCursorStyle(Style{
		back: Black,
		fore: Black,
	})
	for i := 0; i < width*height; i++ {
		t.buf = append(t.buf, ' ')
	}
	t.buf = append(t.buf, []byte("\x1b[H\x1b[2J\x1b[0m\x1b[?25l")...)
}

func (t *Terminal) moveCursor(x, y int) {
	if t.cursor.x == x && t.cursor.y == y {
		return
	}

	if t.cursor.y == y && t.cursor.x < x {
		t.buf = append(t.buf, []byte("\x1b[")...)
		if x-t.cursor.x > 1 {
			t.buf = strconv.AppendInt(t.buf, int64(x-t.cursor.x), 10)
		}
		t.buf = append(t.buf, 'C')
	} else {
		t.buf = append(t.buf, []byte("\x1b[")...)
		t.buf = strconv.AppendInt(t.buf, int64(y)+1, 10)
		t.buf = append(t.buf, ';')
		t.buf = strconv.AppendInt(t.buf, int64(x)+1, 10)
		t.buf = append(t.buf, 'H')
	}

	t.cursor.x = x
	t.cursor.y = y
}

func (t *Terminal) setCursorStyle(s Style) {
	// SGR is short for Select Graphic Rendition
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
		t.buf = strconv.AppendInt(t.buf, int64(sgrs[0]), 10)
		for _, sgr := range sgrs[1:] {
			t.buf = append(t.buf, ';')
			t.buf = strconv.AppendInt(t.buf, int64(sgr), 10)
		}
		t.buf = append(t.buf, 'm')
	}
}

func (t *Terminal) setCursorVisibility(visible bool) {
	if visible && !t.cursor.visible {
		t.buf = append(t.buf, []byte("\x1b[?25h")...)
	} else if !visible && t.cursor.visible {
		t.buf = append(t.buf, []byte("\x1b[?25l")...)
	}
	t.cursor.visible = visible
}

func (t *Terminal) redraw() {
	newScreen := t.render(t.width, t.height)

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
	t.setCursorVisibility(newScreen.cursor.visible)
}

func (t *Terminal) Redraw() error {
	t.redraw()
	return t.flush()
}
