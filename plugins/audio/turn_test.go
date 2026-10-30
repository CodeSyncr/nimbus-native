package audio

import (
	"encoding/binary"
	"math"
	"math/rand"
	"testing"
)

// room is noise at about db decibels below full scale.
func room(rng *rand.Rand, db float64) []int16 {
	out := make([]int16, frame)
	amp := math.Pow(10, db/20) * 32768 * 1.7
	for i := range out {
		out[i] = int16((rng.Float64()*2 - 1) * amp)
	}
	return out
}

// voice is a tone at about db, for one frame.
func voice(db float64, at int) []int16 {
	out := make([]int16, frame)
	amp := math.Pow(10, db/20) * 32768 * math.Sqrt2
	for i := range out {
		out[i] = int16(amp * math.Sin(2*math.Pi*220*float64(at+i)/Rate))
	}
	return out
}

// feed runs seconds of frames from make through d, and returns the first
// event that isn't Nothing with how far in it came (seconds).
func feed(d *TurnDetector, seconds float64, make func(i int) []int16) (Turn, float64, []byte) {
	for i := 0; i < int(seconds*50); i++ {
		if t, _, clip := d.Feed(make(i)); t != Nothing {
			return t, float64(i) / 50, clip
		}
	}
	return Nothing, seconds, nil
}

func samples(wav []byte) int { return (len(wav) - 44) / 2 }

func TestTurn(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	d := NewTurnDetector()
	if ev, _, _ := feed(d, 1, func(int) []int16 { return room(rng, -60) }); ev != Nothing {
		t.Fatalf("a quiet room: %v", ev)
	}
	ev, at, _ := feed(d, 1, func(i int) []int16 { return voice(-25, i*frame) })
	if ev != SpeechStarted || at < 0.15 || at > 0.3 {
		t.Fatalf("speech started: %v after %.2fs", ev, at)
	}
	if ev, _, _ := feed(d, 2, func(i int) []int16 { return voice(-25, i*frame) }); ev != Nothing {
		t.Fatalf("while talking: %v", ev)
	}
	ev, at, clip := feed(d, 3, func(int) []int16 { return room(rng, -60) })
	if ev != TurnEnded || math.Abs(at-endSeconds) > 0.05 {
		t.Fatalf("turn ended: %v after %.2fs", ev, at)
	}
	// About 2.2 s of voice, the lead-in, and tailSeconds of quiet.
	if got := float64(samples(clip)) / Rate; got < 2.6 || got > 3.4 {
		t.Fatalf("clip is %.2fs", got)
	}
	if ev, _, _ := d.Feed(voice(-25, 0)); ev != Nothing {
		t.Fatalf("after the turn: %v", ev)
	}
}

func TestLongPauseEarlyInATurn(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	d := NewTurnDetector()
	// Straight into a word, so there is no lead-in to count.
	feed(d, 0.3, func(i int) []int16 { return voice(-25, i*frame) })
	feed(d, 0.1, func(i int) []int16 { return voice(-25, i*frame) })
	feed(d, 0.5, func(int) []int16 { return room(rng, -60) })
	// A pause past the usual end doesn't end it this early.
	if ev, _, _ := feed(d, 0.6, func(int) []int16 { return room(rng, -60) }); ev != Nothing {
		t.Fatalf("thinking pause: %v", ev)
	}
}

func TestVoiceAcrossTheRoomDoesNotStartATurn(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	d := NewTurnDetector()
	feed(d, 1, func(int) []int16 { return room(rng, -60) })
	if ev, _, _ := feed(d, 3, func(i int) []int16 { return voice(-50, i*frame) }); ev != Nothing {
		t.Fatalf("a far voice: %v", ev)
	}
}

func TestClatterDoesNotStartATurn(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	d := NewTurnDetector()
	feed(d, 1, func(int) []int16 { return room(rng, -60) })
	if ev, _, _ := feed(d, 0.1, func(i int) []int16 { return voice(-20, i*frame) }); ev != Nothing {
		t.Fatalf("a clatter: %v", ev)
	}
	if ev, _, _ := feed(d, 2, func(int) []int16 { return room(rng, -60) }); ev != Nothing {
		t.Fatalf("after the clatter: %v", ev)
	}
}

func TestRoomLowerThanTheSpeakerEndsTheTurn(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	d := NewTurnDetector()
	feed(d, 1, func(int) []int16 { return room(rng, -60) })
	feed(d, 2, func(i int) []int16 { return voice(-20, i*frame) })
	// Chatter well below them, above the old floor, doesn't keep it open.
	ev, at, _ := feed(d, 3, func(int) []int16 { return room(rng, -44) })
	if ev != TurnEnded || at > 1.1 {
		t.Fatalf("chatter: %v after %.2fs", ev, at)
	}
}

func TestNothingSaid(t *testing.T) {
	rng := rand.New(rand.NewSource(6))
	d := NewTurnDetector()
	ev, at, _ := feed(d, 12, func(int) []int16 { return room(rng, -60) })
	if ev != NothingSaid || math.Abs(at-waitSeconds) > 0.05 {
		t.Fatalf("waited: %v after %.2fs", ev, at)
	}
}

func TestHardLimit(t *testing.T) {
	rng := rand.New(rand.NewSource(8))
	// Speech has gaps, which keep the floor down.
	talking := func(i int) []int16 {
		if i%5 == 0 {
			return room(rng, -60)
		}
		return voice(-25, i*frame)
	}
	d := NewTurnDetector()
	feed(d, 1, func(int) []int16 { return room(rng, -60) })
	feed(d, 1, talking)
	ev, at, clip := feed(d, 20, talking)
	if ev != TurnEnded || at > maxTurnSeconds {
		t.Fatalf("limit: %v after %.2fs", ev, at)
	}
	if float64(samples(clip))/Rate > maxTurnSeconds+0.1 {
		t.Fatalf("clip is %d samples", samples(clip))
	}
}

func TestFinishEarly(t *testing.T) {
	if NewTurnDetector().Finish() != nil {
		t.Fatal("a clip from nothing")
	}
	rng := rand.New(rand.NewSource(9))
	d := NewTurnDetector()
	feed(d, 1, func(int) []int16 { return room(rng, -60) })
	feed(d, 1, func(i int) []int16 { return voice(-25, i*frame) })
	if d.Finish() == nil {
		t.Fatal("no clip")
	}
	if d.Finish() != nil {
		t.Fatal("two clips")
	}
}

func TestGateTurnsDownOnlyTheRoom(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	var in []int16
	for i := 0; i < 50; i++ {
		in = append(in, voice(-20, i*frame)...)
	}
	for i := 0; i < 50; i++ {
		in = append(in, room(rng, -50)...)
	}
	out := gate(in, -20)
	if got := loudness(out[:Rate/2]); math.Abs(got-loudness(in[:Rate/2])) > 0.1 {
		t.Fatalf("the voice changed: %.1f dB", got)
	}
	drop := loudness(in[len(in)-Rate/4:]) - loudness(out[len(out)-Rate/4:])
	if math.Abs(drop-10.4) > 1 { // 0.3 of the level
		t.Fatalf("the room dropped %.1f dB", drop)
	}
}

func TestWAV(t *testing.T) {
	w := WAV([]int16{1, -2})
	if string(w[:4]) != "RIFF" || string(w[8:16]) != "WAVEfmt " || string(w[36:40]) != "data" || len(w) != 48 {
		t.Fatalf("header: %q", w)
	}
	if got := binary.LittleEndian.Uint32(w[24:]); got != Rate {
		t.Fatalf("rate %d", got)
	}
	if got := int16(binary.LittleEndian.Uint16(w[46:])); got != -2 {
		t.Fatalf("sample %d", got)
	}
}
