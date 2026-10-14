package main

import "testing"

func TestChordQualitiesAndAccidentals(t *testing.T) {
	for _, symbol := range []string{"C", "F#", "Bb", "Am", "C7", "Fmaj7", "G#m7", "Dbsus2", "Asus4"} {
		ch, err := parseChord(symbol)
		if err != nil {
			t.Errorf("parseChord(%q): %v", symbol, err)
			continue
		}
		if len(guitarVoicing(ch)) < 4 {
			t.Errorf("voicing for %q is too small", symbol)
		}
	}
	for _, symbol := range []string{"Cadd9", "G/B", "H", "Cdim", ""} {
		if _, err := parseChord(symbol); err == nil {
			t.Errorf("parseChord(%q) unexpectedly succeeded", symbol)
		}
	}
}

func TestChordGridDurations(t *testing.T) {
	bars, err := parseChordGrid("| Am | F C | G*2 |")
	if err != nil {
		t.Fatal(err)
	}
	if len(bars) != 4 {
		t.Fatalf("got %d bars, want 4", len(bars))
	}
	if len(bars[1].Chords) != 2 || bars[1].Chords[0].Beats != 2 || bars[1].Chords[1].StartBeat != 2 {
		t.Fatalf("unexpected divided bar: %+v", bars[1])
	}
	if bars[2].Chords[0].Symbol != "G" || bars[3].Chords[0].Symbol != "G" {
		t.Fatal("multi-bar chord was not expanded")
	}
}

func TestChordGridRejectsAmbiguousMultiplier(t *testing.T) {
	if _, err := parseChordGrid("| C G*2 |"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestRhythms(t *testing.T) {
	for input, want := range map[string]string{"whole": "D...", "quarters": "DDDD", "eighths": "DUDUDUDU", "folk": "D.DU.UDU", "d.u.": "D.U."} {
		got, err := rhythmPattern(input)
		if err != nil || got != want {
			t.Errorf("rhythmPattern(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	for _, input := range []string{"D.U", "D-X.", "DDDDDD"} {
		if _, err := rhythmPattern(input); err == nil {
			t.Errorf("rhythmPattern(%q) unexpectedly succeeded", input)
		}
	}
}
