package main

import (
	"sync"
	"testing"
	"time"
)

func TestSequenceTimingAndCountIn(t *testing.T) {
	bars, _ := parseChordGrid("| C | F G |")
	song := defaultSong()
	song.Tempo, song.Rhythm, song.Repeat, song.CountIn, song.Bars = 120, "quarters", 2, 1, bars
	seq, err := buildSequence(song)
	if err != nil {
		t.Fatal(err)
	}
	if seq.Bars != 4 || seq.BarsPerPass != 2 {
		t.Fatalf("unexpected bars: %+v", seq)
	}
	if seq.CountInDuration != 2*time.Second || seq.SongDuration != 8*time.Second || seq.TotalDuration != 10*time.Second {
		t.Fatalf("unexpected durations: count-in=%v song=%v total=%v", seq.CountInDuration, seq.SongDuration, seq.TotalDuration)
	}
	if got := seq.Spans[1]; got.Symbol != "F" || got.StartBeat != 4 || got.EndBeat != 6 {
		t.Fatalf("unexpected span: %+v", got)
	}
	if seq.Events[len(seq.Events)-1].Kind != End {
		t.Fatal("last event is not End")
	}
}

func TestUpStrumRunsHighToLow(t *testing.T) {
	bars, _ := parseChordGrid("| C |")
	song := defaultSong()
	song.Bars, song.Rhythm, song.CountIn, song.Metronome = bars, "U...", 0, false
	seq, err := buildSequence(song)
	if err != nil {
		t.Fatal(err)
	}
	var ons []Event
	for _, e := range seq.Events {
		if e.Kind == NoteOn && e.Channel == 0 {
			ons = append(ons, e)
		}
	}
	if len(ons) < 2 || ons[0].Note <= ons[1].Note {
		t.Fatalf("up-strum notes are not descending: %+v", ons)
	}
}

type fakeSynth struct {
	mu     sync.Mutex
	events []Event
	allOff int
}

func (f *fakeSynth) Send(e Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, e)
	return nil
}
func (f *fakeSynth) AllNotesOff() error { f.mu.Lock(); defer f.mu.Unlock(); f.allOff++; return nil }
func (f *fakeSynth) Close() error       { return nil }

func TestTransportRestartPauseAndSeek(t *testing.T) {
	bars, _ := parseChordGrid("| C | G |")
	song := defaultSong()
	song.Tempo, song.CountIn, song.Metronome, song.Bars = 300, 0, false, bars
	seq, _ := buildSequence(song)
	fake := &fakeSynth{}
	tr := newTransport(seq, fake)
	tr.Start()
	tr.TogglePause()
	if !tr.Snapshot().Paused {
		t.Fatal("transport did not pause")
	}
	tr.SeekBars(1)
	if got := tr.Snapshot().Bar; got != 2 {
		t.Fatalf("seek landed on bar %d", got)
	}
	tr.Restart()
	if snap := tr.Snapshot(); snap.Paused || snap.Bar != 1 {
		t.Fatalf("bad restart snapshot: %+v", snap)
	}
	tr.Stop()
	<-tr.Done()
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.allOff < 4 {
		t.Fatalf("all-notes-off called %d times", fake.allOff)
	}
}
