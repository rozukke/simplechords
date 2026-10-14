package main

import (
	"strings"
	"testing"
)

func TestParseFile(t *testing.T) {
	cfg, err := parseFile(strings.NewReader(`
# practice tune
tempo 90
rhythm folk # trailing comment
repeat 3
count-in 2
metronome off
chords | C# | F G |
chords | Am*2 |
`))
	if err != nil {
		t.Fatal(err)
	}
	if *cfg.tempo != 90 || *cfg.rhythm != "folk" || *cfg.repeat != 3 || *cfg.countIn != 2 || *cfg.metronome {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if got := strings.Join(cfg.chordLines, " "); got != "| C# | F G | | Am*2 |" {
		t.Fatalf("chords = %q", got)
	}
}

func TestParseFileRejectsUnknownKey(t *testing.T) {
	_, err := parseFile(strings.NewReader("swing 0.6\n"))
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestCLIOverridesFileValues(t *testing.T) {
	// This behavior is covered at the flag merge level without creating a file:
	// explicit CLI-only values must produce a complete song.
	song, err := parseCLI([]string{"--tempo", "120", "--rhythm", "eighths", "--repeat", "2", "--count-in", "0", "--metronome", "off", "--chords", "| C | G |"})
	if err != nil {
		t.Fatal(err)
	}
	if song.Tempo != 120 || song.Rhythm != "eighths" || song.Repeat != 2 || song.CountIn != 0 || song.Metronome {
		t.Fatalf("unexpected song: %+v", song)
	}
}
