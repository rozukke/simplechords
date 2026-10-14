package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type ParsedChord struct {
	Symbol    string
	RootPitch int
	Intervals []int
}

var chordRE = regexp.MustCompile(`^([A-Ga-g])([#b]?)(maj7|m7|sus2|sus4|7|m)?$`)

func parseChord(symbol string) (ParsedChord, error) {
	m := chordRE.FindStringSubmatch(strings.TrimSpace(symbol))
	if m == nil {
		return ParsedChord{}, fmt.Errorf("unsupported chord %q", symbol)
	}
	rootNames := map[byte]int{'C': 0, 'D': 2, 'E': 4, 'F': 5, 'G': 7, 'A': 9, 'B': 11}
	letter := byte(strings.ToUpper(m[1])[0])
	root := rootNames[letter]
	if m[2] == "#" {
		root++
	}
	if m[2] == "b" {
		root--
	}
	root = (root + 12) % 12
	var intervals []int
	switch m[3] {
	case "":
		intervals = []int{0, 4, 7}
	case "m":
		intervals = []int{0, 3, 7}
	case "7":
		intervals = []int{0, 4, 7, 10}
	case "maj7":
		intervals = []int{0, 4, 7, 11}
	case "m7":
		intervals = []int{0, 3, 7, 10}
	case "sus2":
		intervals = []int{0, 2, 7}
	case "sus4":
		intervals = []int{0, 5, 7}
	}
	normalized := strings.ToUpper(m[1]) + m[2] + m[3]
	return ParsedChord{Symbol: normalized, RootPitch: root, Intervals: intervals}, nil
}

func guitarVoicing(ch ParsedChord) []uint8 {
	// Put the root between E2 and B2, then spread chord tones through a
	// guitar-like register. These are deliberately simple, stable voicings.
	root := 40 + ((ch.RootPitch - 4 + 12) % 12)
	voices := make([]uint8, 0, 6)
	last := root - 1
	for i := 0; i < 6; i++ {
		interval := ch.Intervals[i%len(ch.Intervals)] + 12*(i/len(ch.Intervals))
		n := root + interval
		for n <= last {
			n += 12
		}
		if n > 76 {
			break
		}
		voices = append(voices, uint8(n))
		last = n
	}
	return voices
}

func parseChordGrid(text string) ([]Bar, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("no chord grid supplied")
	}
	parts := strings.Split(text, "|")
	var bars []Bar
	for _, raw := range parts {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		tokens := strings.Fields(raw)
		if len(tokens) == 0 {
			continue
		}
		if len(tokens) == 1 && strings.Contains(tokens[0], "*") {
			symbol, countText, ok := strings.Cut(tokens[0], "*")
			if !ok || strings.Contains(countText, "*") {
				return nil, fmt.Errorf("invalid multi-bar chord %q", tokens[0])
			}
			count, err := strconv.Atoi(countText)
			if err != nil || count < 1 {
				return nil, fmt.Errorf("invalid bar count in %q", tokens[0])
			}
			ch, err := parseChord(symbol)
			if err != nil {
				return nil, err
			}
			for i := 0; i < count; i++ {
				bars = append(bars, Bar{Chords: []TimedChord{{Symbol: ch.Symbol, Beats: 4}}})
			}
			continue
		}
		for _, token := range tokens {
			if strings.Contains(token, "*") {
				return nil, fmt.Errorf("%q: *N is only valid for a chord alone in its bar", token)
			}
		}
		if len(tokens) > 4 {
			return nil, fmt.Errorf("a bar may contain at most four chords")
		}
		beatLen := 4.0 / float64(len(tokens))
		bar := Bar{}
		for i, token := range tokens {
			ch, err := parseChord(token)
			if err != nil {
				return nil, err
			}
			bar.Chords = append(bar.Chords, TimedChord{Symbol: ch.Symbol, StartBeat: float64(i) * beatLen, Beats: beatLen})
		}
		bars = append(bars, bar)
	}
	if len(bars) == 0 {
		return nil, fmt.Errorf("chord grid contains no bars")
	}
	return bars, nil
}

func rhythmPattern(name string) (string, error) {
	presets := map[string]string{
		"whole": "D...", "quarters": "DDDD", "eighths": "DUDUDUDU", "folk": "D.DU.UDU",
	}
	pattern := strings.ToUpper(strings.TrimSpace(name))
	if p, ok := presets[strings.ToLower(pattern)]; ok {
		pattern = p
	}
	if len(pattern) != 4 && len(pattern) != 8 && len(pattern) != 16 {
		return "", fmt.Errorf("rhythm must be a preset or a 4, 8, or 16 slot D/U/. pattern")
	}
	for _, c := range pattern {
		if c != 'D' && c != 'U' && c != '.' {
			return "", fmt.Errorf("invalid rhythm character %q", c)
		}
	}
	return pattern, nil
}
