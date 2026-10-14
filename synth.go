package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type Synth interface {
	Send(Event) error
	AllNotesOff() error
	Close() error
}

type FluidSynth struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	w     *bufio.Writer
	mu    sync.Mutex
}

func findSoundFont(explicit string) (string, error) {
	if explicit != "" {
		if info, err := os.Stat(explicit); err == nil && !info.IsDir() {
			return explicit, nil
		}
		return "", fmt.Errorf("soundfont not found: %s", explicit)
	}
	candidates := []string{
		"/usr/share/sounds/sf2/FluidR3_GM.sf2",
		"/usr/share/sounds/sf2/FluidR3_GS.sf2",
		"/usr/share/sounds/sf2/default-GM.sf2",
		"/usr/share/soundfonts/FluidR3_GM.sf2",
	}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", errors.New("no SoundFont found; install fluid-soundfont-gm or use --soundfont PATH")
}

func startFluidSynth(soundFont string) (*FluidSynth, error) {
	if _, err := exec.LookPath("fluidsynth"); err != nil {
		return nil, errors.New("fluidsynth is not installed; install the fluidsynth package")
	}
	sf, err := findSoundFont(soundFont)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("fluidsynth", "-q", "-s", "-g", "1.0", filepath.Clean(sf))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start fluidsynth: %w", err)
	}
	f := &FluidSynth{cmd: cmd, stdin: stdin, w: bufio.NewWriter(stdin)}
	// General MIDI program 25 (zero-based) is steel-string guitar.
	if err := f.command("prog 0 25"); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

func (f *FluidSynth) command(format string, args ...any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, err := fmt.Fprintf(f.w, format+"\n", args...); err != nil {
		return err
	}
	return f.w.Flush()
}

func (f *FluidSynth) Send(e Event) error {
	switch e.Kind {
	case NoteOn:
		return f.command("noteon %d %d %d", e.Channel, e.Note, e.Velocity)
	case NoteOff:
		return f.command("noteoff %d %d", e.Channel, e.Note)
	default:
		return nil
	}
}

func (f *FluidSynth) AllNotesOff() error {
	// CC 123 on the two channels used by this program.
	if err := f.command("cc 0 123 0"); err != nil {
		return err
	}
	return f.command("cc 9 123 0")
}

func (f *FluidSynth) Close() error {
	_ = f.AllNotesOff()
	_ = f.command("quit")
	_ = f.stdin.Close()
	done := make(chan error, 1)
	go func() { done <- f.cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(time.Second):
		_ = f.cmd.Process.Kill()
		return <-done
	}
}
