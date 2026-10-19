package main

import (
	"bytes"
	"slices"
	"testing"
)

func TestFluidSynthLogWriterFiltersOnlyDriverBanner(t *testing.T) {
	var output bytes.Buffer
	writer := &fluidSynthLogWriter{target: &output}
	_, _ = writer.Write([]byte("fluidsynth: Using Pulse"))
	_, _ = writer.Write([]byte("Audio driver\nfluidsynth: error: failed to open audio\n"))
	writer.Flush()
	if got, want := output.String(), "fluidsynth: error: failed to open audio\n"; got != want {
		t.Fatalf("logged output = %q, want %q", got, want)
	}
}

func TestFluidSynthUsesPulseWithoutMIDIServer(t *testing.T) {
	args := fluidSynthArgs("/tmp/guitar.sf2")
	if slices.Contains(args, "-s") {
		t.Fatalf("MIDI server must remain disabled: %v", args)
	}
	want := []string{
		"-q",
		"-n",
		"-a", "pulseaudio",
		"-o", "audio.realtime-prio=0",
		"-g", "1.0",
		"/tmp/guitar.sf2",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("fluidSynthArgs() = %v, want %v", args, want)
	}
}
