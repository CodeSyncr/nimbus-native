package audio

import (
	"bytes"
	"encoding/binary"
	"math"
	"sort"
)

// Rate is the microphone's sample rate: 16-bit mono at 16 kHz.
const Rate = 16000

// The detector's settings, in decibels and seconds.
const (
	startMargin    = 11.0 // speech stands this far above the room's floor…
	startSeconds   = 0.22 // …for this long to start a turn
	endMargin      = 6.0  // a turn ends after this long within this of the floor…
	endSeconds     = 0.95
	openingSeconds = 1.6 // …or, early in a turn, a longer pause: often a pause to think
	openingPause   = 1.4
	maxTurnSeconds = 15.0
	waitSeconds    = 10.0 // nothing said after this long: give up
	prerollSeconds = 0.6  // kept from before speech is confirmed, so the first word is whole
	absoluteFloor  = -55.0
	nearSpeech     = -47.0 // a voice must reach this to start a turn: someone at the phone
	otherVoice     = 20.0  // this far below the speaker doesn't keep a turn open
	gateMargin     = 24.0  // in the clip, this far below the speaker is turned down…
	gateGain       = 0.3   // …to this
	gateHold       = 0.25
	tailSeconds    = 0.35 // quiet kept at the end, so a soft last syllable survives
	floorWindow    = 3.0  // seconds of loudness the floor is taken from
	minTurnSeconds = 0.3
	levelFloor     = -55.0 // Level's scale: this is 0…
	levelSpan      = 45.0  // …and this much louder is 1
)

// Turn is what a TurnDetector saw in the frame it was fed.
type Turn int

const (
	Nothing       Turn = iota
	SpeechStarted      // someone started talking
	TurnEnded          // they stopped; the clip comes with it
	NothingSaid        // the wait ran out, or the turn was too short to send
)

type span struct{ db, seconds float64 }

// TurnDetector decides when a spoken turn starts and ends, against the
// room's own noise and the speaker's own voice.
//
// The noise floor is a low percentile of the last few seconds' loudness, so
// it follows a room that gets louder and quieter again, and steady noise
// sinks below it. Speech is sound clearly above the floor and loud enough
// to be close to the phone, for long enough that a clatter does not count.
// Once someone is talking the detector learns how loud they are: a turn
// ends after a short quiet for them (the room may still be chattering well
// below), or at a hard limit. The clip keeps a little of the moment before
// speech began, and what is well below the speaker between their words is
// turned down. Feed it short frames (Listener uses 20 ms).
type TurnDetector struct {
	history  []span
	preroll  []int16
	turn     []int16
	speaking bool
	voiced   float64
	quiet    float64
	waited   float64
	finished bool
	floor    float64
	speaker  *float64 // how loud the person talking is; nil until they start
}

// NewTurnDetector starts listening for one turn.
func NewTurnDetector() *TurnDetector { return &TurnDetector{floor: -50} }

// Feed takes the next frame and reports what it brought, with how loud it
// was, 0…1. With TurnEnded comes the clip, a WAV.
func (d *TurnDetector) Feed(samples []int16) (turn Turn, level float32, clip []byte) {
	if d.finished || len(samples) == 0 {
		return Nothing, 0, nil
	}
	seconds := float64(len(samples)) / Rate
	db := loudness(samples)
	lvl := float32(math.Max(0, math.Min(1, (db-levelFloor)/levelSpan)))
	d.learnFloor(db, seconds)

	loud := db > math.Max(d.floor+startMargin, nearSpeech)
	near := db < d.floor+endMargin || db < absoluteFloor
	if d.speaker != nil && db < *d.speaker-otherVoice {
		near = true
	}

	if !d.speaking {
		d.preroll = append(d.preroll, samples...)
		if keep := int(prerollSeconds * Rate); len(d.preroll) > keep {
			d.preroll = append(d.preroll[:0], d.preroll[len(d.preroll)-keep:]...)
		}
		// Between syllables the count falls back only half as fast as it
		// builds, so speech is confirmed early even in a noisy room.
		if loud {
			d.voiced += seconds
			d.learnSpeaker(db)
		} else {
			d.voiced = math.Max(0, d.voiced-seconds*0.5)
			if d.voiced == 0 {
				d.speaker = nil
			}
		}
		d.waited += seconds
		if d.voiced >= startSeconds {
			d.speaking = true
			d.quiet = 0
			d.turn = append([]int16(nil), d.preroll...)
			return SpeechStarted, lvl, nil
		}
		if d.waited >= waitSeconds {
			d.finished = true
			return NothingSaid, 0, nil
		}
		return Nothing, lvl * 0.5, nil
	}

	d.turn = append(d.turn, samples...)
	if near {
		d.quiet += seconds
	} else {
		d.quiet = 0
		d.learnSpeaker(db)
	}
	length := float64(len(d.turn)) / Rate
	pause := endSeconds
	if length < openingSeconds {
		pause = openingPause
	}
	if d.quiet >= pause || length >= maxTurnSeconds {
		if wav := d.Finish(); wav != nil {
			return TurnEnded, 0, wav
		}
		return NothingSaid, 0, nil
	}
	return Nothing, lvl, nil
}

// Finish ends the turn now and returns it as a WAV, trailing silence
// trimmed; nil if nothing was said.
func (d *TurnDetector) Finish() []byte {
	if d.finished {
		return nil
	}
	d.finished = true
	if !d.speaking || len(d.turn) == 0 {
		return nil
	}
	trim := int(math.Max(0, d.quiet-tailSeconds) * Rate)
	kept := d.turn[:max(0, len(d.turn)-trim)]
	if len(kept) <= int(minTurnSeconds*Rate) {
		return nil
	}
	if d.speaker != nil {
		kept = gate(kept, *d.speaker)
	}
	return WAV(kept)
}

// learnSpeaker follows the speaker's level: up quickly as they get louder,
// down slowly, so one soft word does not lower the bar for the room.
func (d *TurnDetector) learnSpeaker(db float64) {
	if d.speaker == nil {
		d.speaker = &db
		return
	}
	k := 0.08
	if db > *d.speaker {
		k = 0.3
	}
	*d.speaker += (db - *d.speaker) * k
}

// learnFloor takes the 10th percentile of the last few seconds: the room
// between words.
func (d *TurnDetector) learnFloor(db, seconds float64) {
	d.history = append(d.history, span{db, seconds})
	total := 0.0
	for _, s := range d.history {
		total += s.seconds
	}
	for total > floorWindow && len(d.history) > 1 {
		total -= d.history[0].seconds
		d.history = d.history[1:]
	}
	sorted := make([]float64, len(d.history))
	for i, s := range d.history {
		sorted[i] = s.db
	}
	sort.Float64s(sorted)
	d.floor = sorted[min(len(sorted)-1, len(sorted)/10)]
}

// gate turns down what is well below the speaker in 10 ms steps, ramped so
// nothing clicks. The speaker's words pass untouched.
func gate(samples []int16, speaker float64) []int16 {
	step := Rate / 100
	threshold := speaker - gateMargin
	holdSteps := int(gateHold * 100)
	out := make([]int16, len(samples))
	gain, held := 1.0, 0
	for start := 0; start < len(samples); start += step {
		end := min(start+step, len(samples))
		if loudness(samples[start:end]) >= threshold {
			held = holdSteps
		} else if held > 0 {
			held--
		}
		want := gateGain
		if held > 0 {
			want = 1
		}
		// Open within a step, close over a few, so a word's start and tail
		// survive.
		next := want
		if want <= gain {
			next = math.Max(want, gain-0.25)
		}
		n := float64(end - start)
		for i := start; i < end; i++ {
			g := gain + (next-gain)*float64(i-start+1)/n
			out[i] = int16(math.Max(-32768, math.Min(32767, float64(samples[i])*g)))
		}
		gain = next
	}
	return out
}

// loudness is a frame's level in decibels below full scale.
func loudness(samples []int16) float64 {
	sum := 0.0
	for _, s := range samples {
		v := float64(s) / 32768
		sum += v * v
	}
	return 20 * math.Log10(math.Max(math.Sqrt(sum/float64(len(samples))), 1e-6))
}

// WAV wraps 16 kHz mono samples in a RIFF header.
func WAV(samples []int16) []byte {
	var b bytes.Buffer
	le := binary.LittleEndian
	size := uint32(len(samples) * 2)
	b.WriteString("RIFF")
	binary.Write(&b, le, 36+size)
	b.WriteString("WAVEfmt ")
	binary.Write(&b, le, uint32(16))
	binary.Write(&b, le, uint16(1)) // PCM
	binary.Write(&b, le, uint16(1)) // mono
	binary.Write(&b, le, uint32(Rate))
	binary.Write(&b, le, uint32(Rate*2))
	binary.Write(&b, le, uint16(2))
	binary.Write(&b, le, uint16(16))
	b.WriteString("data")
	binary.Write(&b, le, size)
	binary.Write(&b, le, samples)
	return b.Bytes()
}
