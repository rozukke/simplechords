# simplechords

`simplechords` is a small terminal practice player. It plays one
strummed guitar part, intended to replace an instructor for backing parts.

## Install

Build with Go 1.22 or newer:

```sh
go build -o simplechords .
```

Playback uses FluidSynth and a General MIDI SoundFont. On Debian or Ubuntu:

```sh
sudo apt install fluidsynth fluid-soundfont-gm
```

If your SoundFont is not in a standard Debian/Ubuntu location, pass
`--soundfont /path/to/file.sf2` or set `SOUNDFONT`.

## Use

Play the included example:

```sh
./simplechords example.chords

# Override a setting from the song file for this playback.
./simplechords example.chords --tempo 100
```

Or create a progression directly:

```sh
./simplechords --tempo 90 --rhythm folk --repeat 4 \
  --chords '| Am | F C | G*2 |'
```

A song file is plain text:

```text
tempo 90
rhythm folk
repeat 4
count-in 1
metronome on
chords | Am | F C | G*2 |
```

Settings given as flags override the file. Multiple `chords` lines are joined,
which makes long progressions easier to read. Lines beginning with `#` and
trailing comments are ignored.

## Notation

Bars are enclosed by `|`. One chord fills a bar; multiple chords split it
evenly. A chord alone may use `*N` to last several bars:

```text
chords | C | Am F | G*2 |
```

Supported chord qualities are major, minor, dominant seventh, major seventh,
minor seventh, suspended second, and suspended fourth. Roots may be natural,
sharp, or flat: `C`, `F#`, `Bb`, `Am`, `C7`, `Fmaj7`, `G#m7`, `Dsus2`,
`Asus4`.

Built-in rhythms are `whole`, `quarters`, `eighths`, and `folk`. A custom
rhythm is a 4-, 8-, or 16-slot grid. `D` is a down-strum, `U` is an up-strum,
and `.` lets the previous chord ring:

```text
rhythm D.DU.UDU
```

The progression is always in 4/4. `repeat` determines the total number of
passes. The count-in is audible even if the playback metronome is disabled.

## Controls

| Key | Action |
| --- | --- |
| Space | Pause or resume |
| Left / Right | Previous or next bar |
| `r` or Home | Restart from the count-in |
| `q`, Escape, or Ctrl-C | Quit |

Every pause, seek, restart, and exit sends all-notes-off before changing state.

## Troubleshooting

Check the runtime directly with:

```sh
fluidsynth --version
fluidsynth -a help
```

If startup reports that no SoundFont was found, install
`fluid-soundfont-gm` or select one explicitly with `--soundfont`.

## Acknowledgements

This project was inspired by the MIT-licensed
[**Backing Tracks**](https://github.com/ako/backing-tracks) project by
Andrej Koelewijn, but simpler.

## License

`simplechords` is available under the [MIT License](LICENSE).
