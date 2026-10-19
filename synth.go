package main

import (
	"bufio"
	"bytes"
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
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	w      *bufio.Writer
	stderr *fluidSynthLogWriter
	mu     sync.Mutex
}

// fluidSynthLogWriter keeps the terminal UI clean without hiding diagnostics.
// FluidSynth writes its normal driver banner to stderr, so discard just that
// known informational line and pass warnings and errors through unchanged.
type fluidSynthLogWriter struct {
	target  io.Writer
	pending []byte
	mu      sync.Mutex
}

func (w *fluidSynthLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pending = append(w.pending, p...)
	w.writeCompleteLines()
	return len(p), nil
}

func (w *fluidSynthLogWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.pending) > 0 {
		w.writeLine(w.pending)
		w.pending = nil
	}
}

func (w *fluidSynthLogWriter) writeCompleteLines() {
	for {
		i := bytes.IndexByte(w.pending, '\n')
		if i < 0 {
			return
		}
		w.writeLine(w.pending[:i+1])
		w.pending = w.pending[i+1:]
	}
}

func (w *fluidSynthLogWriter) writeLine(line []byte) {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("fluidsynth: Using PulseAudio driver")) {
		return
	}
	_, _ = w.target.Write(line)
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
	cmd := exec.Command("fluidsynth", fluidSynthArgs(sf)...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	// FluidSynth logs its driver banner to stderr even in quiet mode. Filter
	// that one benign line, while retaining errors for the user.
	stderr := &fluidSynthLogWriter{target: os.Stderr}
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start fluidsynth: %w", err)
	}
	f := &FluidSynth{cmd: cmd, stdin: stdin, w: bufio.NewWriter(stdin), stderr: stderr}
	// General MIDI program 25 (zero-based) is steel-string guitar.
	if err := f.command("prog 0 25"); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

func fluidSynthArgs(soundFont string) []string {
	// Commands are sent through FluidSynth's stdin shell, so its MIDI server
	// and MIDI input (-n) is disabled. This avoids probing the ALSA sequencer.
	// PulseAudio also works through pipewire-pulse on current Linux desktops.
	// Normal scheduling is sufficient here and avoids harmless high-priority
	// warnings on systems that do not grant realtime scheduling privileges.
	return []string{
		"-q",
		"-n",
		"-a", "pulseaudio",
		"-o", "audio.realtime-prio=0",
		"-g", "1.0",
		filepath.Clean(soundFont),
	}
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
	var err error
	select {
	case err = <-done:
	case <-time.After(time.Second):
		_ = f.cmd.Process.Kill()
		err = <-done
	}
	f.stderr.Flush()
	return err
}
