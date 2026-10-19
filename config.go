package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const defaultRhythm = "quarters"

type Song struct {
	Tempo         int
	TimeSignature string
	Rhythm        string
	Repeat        int
	CountIn       int
	Metronome     bool
	Bars          []Bar
	SoundFont     string
}

type Bar struct {
	Chords []TimedChord
}

type TimedChord struct {
	Symbol    string
	StartBeat float64
	Beats     float64
}

type fileConfig struct {
	tempo      *int
	time       *string
	rhythm     *string
	repeat     *int
	countIn    *int
	metronome  *bool
	chordLines []string
}

func defaultSong() Song {
	return Song{Tempo: 100, TimeSignature: "4/4", Rhythm: defaultRhythm, Repeat: 1, CountIn: 1, Metronome: true}
}

func parseFile(r io.Reader) (fileConfig, error) {
	var cfg fileConfig
	s := bufio.NewScanner(r)
	for lineNo := 1; s.Scan(); lineNo++ {
		line := stripComment(s.Text())
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, value, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok || strings.TrimSpace(value) == "" {
			return cfg, fmt.Errorf("line %d: expected 'key value'", lineNo)
		}
		value = strings.TrimSpace(value)
		switch strings.ToLower(key) {
		case "tempo":
			v, err := strconv.Atoi(value)
			if err != nil {
				return cfg, fmt.Errorf("line %d: invalid tempo %q", lineNo, value)
			}
			cfg.tempo = &v
		case "time":
			if _, err := parseTimeSignature(value); err != nil {
				return cfg, fmt.Errorf("line %d: %w", lineNo, err)
			}
			cfg.time = &value
		case "rhythm":
			cfg.rhythm = &value
		case "repeat":
			v, err := strconv.Atoi(value)
			if err != nil {
				return cfg, fmt.Errorf("line %d: invalid repeat %q", lineNo, value)
			}
			cfg.repeat = &v
		case "count-in", "count_in":
			v, err := strconv.Atoi(value)
			if err != nil {
				return cfg, fmt.Errorf("line %d: invalid count-in %q", lineNo, value)
			}
			cfg.countIn = &v
		case "metronome":
			v, err := parseOnOff(value)
			if err != nil {
				return cfg, fmt.Errorf("line %d: %w", lineNo, err)
			}
			cfg.metronome = &v
		case "chords":
			cfg.chordLines = append(cfg.chordLines, value)
		default:
			return cfg, fmt.Errorf("line %d: unknown setting %q", lineNo, key)
		}
	}
	if err := s.Err(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func stripComment(line string) string {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "#") {
		return ""
	}
	for i := 1; i < len(line); i++ {
		if line[i] == '#' && (line[i-1] == ' ' || line[i-1] == '\t') {
			return strings.TrimSpace(line[:i])
		}
	}
	return line
}

func parseOnOff(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "on", "true", "yes":
		return true, nil
	case "off", "false", "no":
		return false, nil
	default:
		return false, fmt.Errorf("expected on or off, got %q", s)
	}
}

// reorderCLIArgs permits options on either side of the optional song file. The
// standard flag package stops parsing at its first positional argument, while
// commands such as "simplechords song.chords --tempo 100" are expected to work.
func reorderCLIArgs(args []string) []string {
	valueFlags := map[string]bool{
		"tempo": true, "time": true, "rhythm": true, "repeat": true, "count-in": true,
		"metronome": true, "chords": true, "soundfont": true,
	}
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			return append(append(flags, "--"), positional...)
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			continue
		}

		flags = append(flags, arg)
		name := strings.TrimLeft(arg, "-")
		name, _, hasValue := strings.Cut(name, "=")
		if valueFlags[name] && !hasValue && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, positional...)
}

func parseCLI(args []string) (Song, error) {
	fs := flag.NewFlagSet("simplechords", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	tempo := fs.Int("tempo", 0, "tempo in BPM")
	timeSignature := fs.String("time", "", "time signature, e.g. 3/4")
	rhythm := fs.String("rhythm", "", "rhythm preset or D/U/. grid")
	repeat := fs.Int("repeat", 0, "number of passes")
	countIn := fs.Int("count-in", -1, "count-in bars")
	metronome := fs.String("metronome", "", "on or off")
	chords := fs.String("chords", "", "bar-grid chord progression")
	soundfont := fs.String("soundfont", "", "path to a SoundFont")
	if err := fs.Parse(reorderCLIArgs(args)); err != nil {
		return Song{}, err
	}
	if len(fs.Args()) > 1 {
		return Song{}, errors.New("expected at most one song file")
	}

	song := defaultSong()
	var chordText string
	if len(fs.Args()) == 1 {
		f, err := os.Open(fs.Args()[0])
		if err != nil {
			return Song{}, err
		}
		cfg, err := parseFile(f)
		f.Close()
		if err != nil {
			return Song{}, err
		}
		if cfg.tempo != nil {
			song.Tempo = *cfg.tempo
		}
		if cfg.time != nil {
			song.TimeSignature = *cfg.time
		}
		if cfg.rhythm != nil {
			song.Rhythm = *cfg.rhythm
		}
		if cfg.repeat != nil {
			song.Repeat = *cfg.repeat
		}
		if cfg.countIn != nil {
			song.CountIn = *cfg.countIn
		}
		if cfg.metronome != nil {
			song.Metronome = *cfg.metronome
		}
		chordText = strings.Join(cfg.chordLines, " ")
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if set["tempo"] {
		song.Tempo = *tempo
	}
	if set["time"] {
		song.TimeSignature = *timeSignature
	}
	if set["rhythm"] {
		song.Rhythm = *rhythm
	}
	if set["repeat"] {
		song.Repeat = *repeat
	}
	if set["count-in"] {
		song.CountIn = *countIn
	}
	if set["metronome"] {
		v, err := parseOnOff(*metronome)
		if err != nil {
			return Song{}, err
		}
		song.Metronome = v
	}
	if set["chords"] {
		chordText = *chords
	}
	if set["soundfont"] {
		song.SoundFont = *soundfont
	}
	if song.SoundFont == "" {
		song.SoundFont = os.Getenv("SOUNDFONT")
	}

	meter, err := parseTimeSignature(song.TimeSignature)
	if err != nil {
		return Song{}, err
	}
	bars, err := parseChordGridForMeter(chordText, meter.BeatsPerBar)
	if err != nil {
		return Song{}, err
	}
	song.Bars = bars
	if err := validateSong(song); err != nil {
		return Song{}, err
	}
	return song, nil
}

func validateSong(s Song) error {
	if s.Tempo < 30 || s.Tempo > 300 {
		return fmt.Errorf("tempo must be between 30 and 300 BPM")
	}
	if s.Repeat < 1 || s.Repeat > 10000 {
		return fmt.Errorf("repeat must be between 1 and 10000")
	}
	if s.CountIn < 0 || s.CountIn > 4 {
		return fmt.Errorf("count-in must be between 0 and 4 bars")
	}
	if _, err := parseTimeSignature(s.TimeSignature); err != nil {
		return err
	}
	if _, err := rhythmPattern(s.Rhythm); err != nil {
		return err
	}
	if len(s.Bars) == 0 {
		return errors.New("no chords supplied; use a song file or --chords")
	}
	return nil
}

func usage(w io.Writer) {
	fmt.Fprintln(w, `Usage:
  simplechords [options] song.chords
  simplechords --tempo 90 --rhythm folk --repeat 4 --chords '| Am | F C | G*2 |'

Options:
  --tempo BPM          tempo (default 100)
  --time N/D           time signature (default 4/4)
  --rhythm PATTERN     whole, quarters, eighths, folk, or a 4/8/16-slot D/U/. grid
  --repeat N           progression passes (default 1)
  --count-in N         count-in bars, 0-4 (default 1)
  --metronome on|off   audible click (default on)
  --chords GRID        chord bar grid; overrides chords in a file
  --soundfont PATH     GM-compatible .sf2 file (or set SOUNDFONT)

Controls: Space pause/resume, Left/Right seek one bar, r/Home restart, q/Esc quit`)
}
