package main

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/term"
)

type Key uint8

const (
	KeyPause Key = iota
	KeyLeft
	KeyRight
	KeyRestart
	KeyQuit
)

func readKeys(r io.Reader) <-chan Key {
	out := make(chan Key)
	bytes := make(chan byte)
	go func() {
		defer close(bytes)
		buf := make([]byte, 1)
		for {
			n, err := r.Read(buf)
			if n == 1 {
				bytes <- buf[0]
			}
			if err != nil {
				return
			}
		}
	}()
	go func() {
		defer close(out)
		for b := range bytes {
			switch b {
			case ' ':
				out <- KeyPause
			case 'r', 'R':
				out <- KeyRestart
			case 'q', 'Q', 3:
				out <- KeyQuit
			case 27:
				timer := time.NewTimer(30 * time.Millisecond)
				select {
				case b2, ok := <-bytes:
					if !ok {
						timer.Stop()
						out <- KeyQuit
						return
					}
					if b2 != '[' {
						timer.Stop()
						out <- KeyQuit
						continue
					}
					select {
					case b3 := <-bytes:
						timer.Stop()
						switch b3 {
						case 'D':
							out <- KeyLeft
						case 'C':
							out <- KeyRight
						case 'H':
							out <- KeyRestart
						case '1':
							<-bytes
							out <- KeyRestart
						}
					case <-timer.C:
						out <- KeyQuit
					}
				case <-timer.C:
					out <- KeyQuit
				}
			}
		}
	}()
	return out
}

func makeRawTerminal() (func(), error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return func() {}, nil
	}
	old, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	var once sync.Once
	return func() { once.Do(func() { _ = term.Restore(fd, old) }) }, nil
}

func renderStatus(w io.Writer, s Snapshot, bars, repeats int) {
	state := "playing"
	if s.Paused {
		state = "paused"
	}
	var position string
	if s.InCountIn {
		position = fmt.Sprintf("count-in %d:%d", s.CountInBar, s.Beat)
	} else {
		position = fmt.Sprintf("bar %d/%d beat %d  pass %d/%d", s.Bar, bars, s.Beat, s.Repeat, repeats)
	}
	fmt.Fprintf(w, "\r\x1b[2K%-7s  %-7s  %-30s  %s/%s", state, s.Chord, position, shortDuration(s.SongPosition), shortDuration(s.Total))
}

func shortDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d.Round(time.Second).Seconds())
	return fmt.Sprintf("%d:%02d", total/60, total%60)
}
