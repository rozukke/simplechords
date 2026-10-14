package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) == 1 {
		usage(os.Stderr)
		return 2
	}
	for _, arg := range os.Args[1:] {
		if arg == "-h" || arg == "--help" {
			usage(os.Stdout)
			return 0
		}
	}
	song, err := parseCLI(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		fmt.Fprintln(os.Stderr, "use --help for usage")
		return 2
	}
	seq, err := buildSequence(song)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}
	synth, err := startFluidSynth(song.SoundFont)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	defer synth.Close()

	restore, err := makeRawTerminal()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: terminal:", err)
		return 1
	}
	defer restore()

	transport := newTransport(seq, synth)
	transport.Start()
	keys := readKeys(os.Stdin)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	exit := false
	for !exit {
		select {
		case key, ok := <-keys:
			if !ok {
				keys = nil
				continue
			}
			switch key {
			case KeyPause:
				transport.TogglePause()
			case KeyLeft:
				transport.SeekBars(-1)
			case KeyRight:
				transport.SeekBars(1)
			case KeyRestart:
				transport.Restart()
			case KeyQuit:
				transport.Stop()
				exit = true
			}
		case <-ticker.C:
			renderStatus(os.Stdout, transport.Snapshot(), seq.Bars, song.Repeat)
		case <-transport.Done():
			exit = true
		case <-signals:
			transport.Stop()
			exit = true
		}
	}
	restore()
	fmt.Fprintln(os.Stdout)
	if err := transport.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}
