package main

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type commandKind uint8

const (
	togglePause commandKind = iota
	seekBars
	restart
	stop
)

type transportCommand struct {
	kind   commandKind
	amount int
}

type Snapshot struct {
	Playing      bool
	Paused       bool
	InCountIn    bool
	CountInBar   int
	Bar          int
	Beat         int
	Repeat       int
	Chord        string
	Position     time.Duration
	SongPosition time.Duration
	Total        time.Duration
	Err          error
}

type Transport struct {
	seq      Sequence
	synth    Synth
	commands chan transportCommand
	queries  chan chan Snapshot
	done     chan struct{}
	once     sync.Once
	errMu    sync.Mutex
	err      error
}

func newTransport(seq Sequence, synth Synth) *Transport {
	return &Transport{seq: seq, synth: synth, commands: make(chan transportCommand), queries: make(chan chan Snapshot), done: make(chan struct{})}
}

func (t *Transport) Start() { go t.run() }

func (t *Transport) Done() <-chan struct{} { return t.done }

func (t *Transport) Err() error {
	t.errMu.Lock()
	defer t.errMu.Unlock()
	return t.err
}

func (t *Transport) setErr(err error) {
	t.errMu.Lock()
	t.err = err
	t.errMu.Unlock()
}

func (t *Transport) command(cmd transportCommand) {
	select {
	case t.commands <- cmd:
	case <-t.done:
	}
}

func (t *Transport) TogglePause()   { t.command(transportCommand{kind: togglePause}) }
func (t *Transport) SeekBars(n int) { t.command(transportCommand{kind: seekBars, amount: n}) }
func (t *Transport) Restart()       { t.command(transportCommand{kind: restart}) }
func (t *Transport) Stop()          { t.command(transportCommand{kind: stop}) }

func (t *Transport) Snapshot() Snapshot {
	reply := make(chan Snapshot, 1)
	select {
	case t.queries <- reply:
		select {
		case s := <-reply:
			return s
		case <-t.done:
			return Snapshot{Total: t.seq.SongDuration}
		}
	case <-t.done:
		return Snapshot{Total: t.seq.SongDuration}
	}
}

func (t *Transport) finish() { t.once.Do(func() { close(t.done) }) }

func (t *Transport) run() {
	defer t.finish()
	position := time.Duration(0)
	startedAt := time.Now()
	playing := true
	paused := false
	next := 0
	currentPosition := func() time.Duration {
		if playing && !paused {
			return position + time.Since(startedAt)
		}
		return position
	}
	setPosition := func(p time.Duration) {
		if p < 0 {
			p = 0
		}
		if p > t.seq.TotalDuration {
			p = t.seq.TotalDuration
		}
		position = p
		startedAt = time.Now()
		next = sort.Search(len(t.seq.Events), func(i int) bool { return t.seq.Events[i].At >= p })
	}

	timer := time.NewTimer(0)
	if !timer.Stop() {
		<-timer.C
	}
	defer timer.Stop()
	var timerC <-chan time.Time
	resetTimer := func() {
		if !playing || paused || next >= len(t.seq.Events) {
			timerC = nil
			return
		}
		wait := t.seq.Events[next].At - currentPosition()
		if wait < 0 {
			wait = 0
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(wait)
		timerC = timer.C
	}
	resetTimer()

	for {
		select {
		case <-timerC:
			pos := currentPosition()
			for next < len(t.seq.Events) && t.seq.Events[next].At <= pos+time.Millisecond {
				e := t.seq.Events[next]
				next++
				if e.Kind == End {
					position = t.seq.TotalDuration
					_ = t.synth.AllNotesOff()
					return
				}
				if err := t.synth.Send(e); err != nil {
					t.setErr(fmt.Errorf("synth playback: %w", err))
					_ = t.synth.AllNotesOff()
					return
				}
			}
			resetTimer()

		case cmd := <-t.commands:
			position = currentPosition()
			startedAt = time.Now()
			switch cmd.kind {
			case togglePause:
				_ = t.synth.AllNotesOff()
				paused = !paused
			case seekBars:
				_ = t.synth.AllNotesOff()
				songStart := t.seq.LeadInDuration + t.seq.CountInDuration
				bar := -1
				if position >= songStart {
					bar = int((position - songStart) / t.seq.BarDuration)
				}
				bar += cmd.amount
				if bar < 0 {
					bar = 0
				}
				if bar >= t.seq.Bars {
					bar = t.seq.Bars - 1
				}
				setPosition(songStart + time.Duration(bar)*t.seq.BarDuration)
			case restart:
				_ = t.synth.AllNotesOff()
				paused = false
				setPosition(0)
			case stop:
				_ = t.synth.AllNotesOff()
				return
			}
			resetTimer()

		case reply := <-t.queries:
			reply <- t.snapshot(currentPosition(), paused, t.Err())
		}
	}
}

func (t *Transport) snapshot(position time.Duration, paused bool, playErr error) Snapshot {
	s := Snapshot{Playing: !paused, Paused: paused, Position: position, Total: t.seq.SongDuration, Err: playErr, Chord: "-"}
	songStart := t.seq.LeadInDuration + t.seq.CountInDuration
	if position < songStart {
		s.InCountIn = true
		if position < t.seq.LeadInDuration {
			// The pre-roll lets the synth start before count-in beat one.
			s.CountInBar, s.Beat = 1, 1
			return s
		}
		countInPos := position - t.seq.LeadInDuration
		s.CountInBar = int(countInPos/t.seq.BarDuration) + 1
		s.Beat = int((countInPos%t.seq.BarDuration)/t.seq.BeatDuration) + 1
		return s
	}
	songPos := position - songStart
	if songPos > t.seq.SongDuration {
		songPos = t.seq.SongDuration
	}
	s.SongPosition = songPos
	beat := float64(songPos) / float64(t.seq.BeatDuration)
	bar := int(beat / float64(t.seq.BeatsPerBar))
	if bar >= t.seq.Bars {
		bar = t.seq.Bars - 1
	}
	s.Bar = bar + 1
	s.Beat = int(beat-float64(bar*t.seq.BeatsPerBar)) + 1
	s.Repeat = bar/t.seq.BarsPerPass + 1
	if span, ok := spanAt(t.seq.Spans, beat); ok {
		s.Chord = span.Symbol
	}
	return s
}
