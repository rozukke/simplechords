package main

import (
	"slices"
	"testing"
)

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
