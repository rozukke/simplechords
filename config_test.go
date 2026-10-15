package main

import (
	"os"
	"path/filepath"
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

func TestCLIParsesOptions(t *testing.T) {
	song, err := parseCLI([]string{"--tempo", "120", "--rhythm", "eighths", "--repeat", "2", "--count-in", "0", "--metronome", "off", "--chords", "| C | G |"})
	if err != nil {
		t.Fatal(err)
	}
	if song.Tempo != 120 || song.Rhythm != "eighths" || song.Repeat != 2 || song.CountIn != 0 || song.Metronome {
		t.Fatalf("unexpected song: %+v", song)
	}
}

func TestCLIFlagsAfterFileOverrideFileValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "song.chords")
	if err := os.WriteFile(path, []byte("tempo 90\nrhythm folk\nrepeat 2\ncount-in 2\nmetronome on\nchords | C | G |\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	song, err := parseCLI([]string{path, "--tempo", "120", "--repeat=4", "--count-in", "0", "--metronome", "off"})
	if err != nil {
		t.Fatal(err)
	}
	if song.Tempo != 120 || song.Rhythm != "folk" || song.Repeat != 4 || song.CountIn != 0 || song.Metronome {
		t.Fatalf("file settings were not overridden: %+v", song)
	}
}
