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

func TestChordGridExplicitDurations(t *testing.T) {
	bars, err := parseChordGridForMeter("| C(2/3) G | C(1/4) Am G |", 3)
	if err != nil {
		t.Fatal(err)
	}
	if got := bars[0].Chords; got[0].Beats != 2 || got[1].StartBeat != 2 || got[1].Beats != 1 {
		t.Fatalf("unexpected 3/4 durations: %+v", got)
	}
	if got := bars[1].Chords; got[0].Beats != .75 || got[1].Beats != 1.125 || got[2].StartBeat != 1.875 {
		t.Fatalf("unexpected mixed durations: %+v", got)
	}
	for _, grid := range []string{"| C(2/3) G(2/3) |", "| C(1/3) G(1/3) |", "| C(0/2) G |", "| C(3/2) |"} {
		if _, err := parseChordGrid(grid); err == nil {
			t.Errorf("parseChordGrid(%q) unexpectedly succeeded", grid)
		}
	}
}

func TestTimeSignatures(t *testing.T) {
	for _, input := range []string{"3/4", "6/8", "12/16"} {
		if _, err := parseTimeSignature(input); err != nil {
			t.Errorf("parseTimeSignature(%q): %v", input, err)
		}
	}
	for _, input := range []string{"4", "0/4", "4/3", "33/4"} {
		if _, err := parseTimeSignature(input); err == nil {
			t.Errorf("parseTimeSignature(%q) unexpectedly succeeded", input)
		}
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
