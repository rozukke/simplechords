package main

import (
	"fmt"
	"sort"
	"time"
)

type EventKind uint8

const (
	NoteOff EventKind = iota
	NoteOn
	End
)

type Event struct {
	At       time.Duration
	Kind     EventKind
	Channel  uint8
	Note     uint8
	Velocity uint8
}

type chordSpan struct {
	Symbol             string
	StartBeat, EndBeat float64
}

type Sequence struct {
	Events          []Event
	Tempo           int
	BeatDuration    time.Duration
	BarDuration     time.Duration
	LeadInDuration  time.Duration
	CountInDuration time.Duration
	SongDuration    time.Duration
	BeatsPerBar     int
	TotalDuration   time.Duration
	Bars            int
	BarsPerPass     int
	Spans           []chordSpan
}

type strumHit struct {
	beat      float64
	direction byte
	chord     ParsedChord
	chordEnd  float64
}

func buildSequence(song Song) (Sequence, error) {
	pattern, err := rhythmPattern(song.Rhythm)
	if err != nil {
		return Sequence{}, err
	}
	meter, err := parseTimeSignature(song.TimeSignature)
	if err != nil {
		return Sequence{}, err
	}
	// Tempo is expressed as quarter-note BPM. A meter's denominator determines
	// the duration of its beat (e.g. an eighth-note beat in 6/8).
	quarterDuration := time.Minute / time.Duration(song.Tempo)
	beatDuration := quarterDuration * 4 / time.Duration(meter.BeatUnit)
	barDuration := beatDuration * time.Duration(meter.BeatsPerBar)
	countInDuration := time.Duration(song.CountIn) * barDuration
	leadInDuration := time.Duration(0)
	if song.CountIn > 0 {
		// Give FluidSynth one beat to start before the first audible count-in.
		leadInDuration = beatDuration
	}

	seq := Sequence{
		Tempo: song.Tempo, BeatDuration: beatDuration, BarDuration: barDuration,
		LeadInDuration: leadInDuration, CountInDuration: countInDuration,
		BeatsPerBar: meter.BeatsPerBar, BarsPerPass: len(song.Bars),
		Bars: len(song.Bars) * song.Repeat,
	}
	var hits []strumHit
	for pass := 0; pass < song.Repeat; pass++ {
		for localBar, bar := range song.Bars {
			barIndex := pass*len(song.Bars) + localBar
			barStart := float64(barIndex * meter.BeatsPerBar)
			for _, c := range bar.Chords {
				parsed, err := parseChord(c.Symbol)
				if err != nil {
					return Sequence{}, err
				}
				seq.Spans = append(seq.Spans, chordSpan{c.Symbol, barStart + c.StartBeat, barStart + c.StartBeat + c.Beats})
				_ = parsed
			}
			for slot, direction := range []byte(pattern) {
				if direction == '.' {
					continue
				}
				beat := barStart + float64(slot)*float64(meter.BeatsPerBar)/float64(len(pattern))
				span, ok := spanAt(seq.Spans, beat)
				if !ok {
					return Sequence{}, fmt.Errorf("no chord at bar %d beat %.2f", barIndex+1, beat-barStart+1)
				}
				parsed, _ := parseChord(span.Symbol)
				hits = append(hits, strumHit{beat: beat, direction: direction, chord: parsed, chordEnd: span.EndBeat})
			}
			// A new chord must be heard even when its boundary falls between
			// rhythm slots. The rhythm's own hit takes precedence at an exact
			// boundary; otherwise add a down-strum for the chord change.
			for _, c := range bar.Chords {
				if c.StartBeat == 0 {
					continue
				}
				beat := barStart + c.StartBeat
				if hasHitAt(hits, beat) {
					continue
				}
				parsed, _ := parseChord(c.Symbol)
				hits = append(hits, strumHit{beat: beat, direction: 'D', chord: parsed, chordEnd: beat + c.Beats})
			}
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].beat < hits[j].beat })

	seq.SongDuration = time.Duration(float64(beatDuration) * float64(seq.Bars*meter.BeatsPerBar))
	seq.TotalDuration = seq.LeadInDuration + seq.CountInDuration + seq.SongDuration

	// Count-in is always audible. The metronome setting controls song clicks.
	for beat := 0; beat < song.CountIn*meter.BeatsPerBar; beat++ {
		seq.addClick(leadInDuration+time.Duration(beat)*beatDuration, beat%meter.BeatsPerBar == 0)
	}
	if song.Metronome {
		for beat := 0; beat < seq.Bars*meter.BeatsPerBar; beat++ {
			seq.addClick(leadInDuration+countInDuration+time.Duration(beat)*beatDuration, beat%meter.BeatsPerBar == 0)
		}
	}

	for i, hit := range hits {
		start := leadInDuration + countInDuration + beatsToDuration(hit.beat, beatDuration)
		endBeat := hit.chordEnd
		if i+1 < len(hits) && hits[i+1].beat < endBeat {
			endBeat = hits[i+1].beat
		}
		end := leadInDuration + countInDuration + beatsToDuration(endBeat, beatDuration) - 5*time.Millisecond
		notes := guitarVoicing(hit.chord)
		if hit.direction == 'U' {
			reverseUint8(notes)
		}
		velocity := uint8(72)
		if int(hit.beat)%meter.BeatsPerBar == 0 {
			velocity = 86
		}
		for stringIndex, note := range notes {
			on := start + time.Duration(stringIndex)*14*time.Millisecond
			off := end
			if off < on+20*time.Millisecond {
				off = on + 20*time.Millisecond
			}
			seq.Events = append(seq.Events,
				Event{At: on, Kind: NoteOn, Channel: 0, Note: note, Velocity: velocity},
				Event{At: off, Kind: NoteOff, Channel: 0, Note: note},
			)
		}
	}
	seq.Events = append(seq.Events, Event{At: seq.TotalDuration, Kind: End})
	sort.SliceStable(seq.Events, func(i, j int) bool {
		if seq.Events[i].At != seq.Events[j].At {
			return seq.Events[i].At < seq.Events[j].At
		}
		return seq.Events[i].Kind < seq.Events[j].Kind
	})
	return seq, nil
}

func (s *Sequence) addClick(at time.Duration, accent bool) {
	note, velocity := uint8(77), uint8(72)
	if accent {
		note, velocity = 76, 100
	}
	s.Events = append(s.Events,
		Event{At: at, Kind: NoteOn, Channel: 9, Note: note, Velocity: velocity},
		Event{At: at + 45*time.Millisecond, Kind: NoteOff, Channel: 9, Note: note},
	)
}

func hasHitAt(hits []strumHit, beat float64) bool {
	const epsilon = 1e-9
	for _, hit := range hits {
		if hit.beat >= beat-epsilon && hit.beat <= beat+epsilon {
			return true
		}
	}
	return false
}

func spanAt(spans []chordSpan, beat float64) (chordSpan, bool) {
	for i := len(spans) - 1; i >= 0; i-- {
		if beat >= spans[i].StartBeat && beat < spans[i].EndBeat {
			return spans[i], true
		}
	}
	return chordSpan{}, false
}

func beatsToDuration(beats float64, beatDuration time.Duration) time.Duration {
	return time.Duration(beats * float64(beatDuration))
}

func reverseUint8(v []uint8) {
	for i, j := 0, len(v)-1; i < j; i, j = i+1, j-1 {
		v[i], v[j] = v[j], v[i]
	}
}
