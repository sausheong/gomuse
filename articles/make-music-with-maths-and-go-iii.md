# Make Music With Maths and Go (III)

*Teaching a computer to play guitar, sing a little, and keep the drummer in time*

In the [first article](https://medium.com/geekculture/make-music-with-maths-and-go-i-abfdbcf73b65), we made musical notes from sine waves. In the [second article](https://medium.com/geekculture/make-music-with-maths-and-go-ii-7d49d438b83c), we put those notes into a score and built a Go application to play them.

Now we have another problem. Our computer knows which notes to play, but we would quite like it to sound as though it has brought some instruments.

A piano and a guitar can play exactly the same note, yet we can usually tell them apart. A singer can join them on that note and remain recognisable too. Meanwhile, the drummer is hitting something that might not have a clear musical pitch at all.

How do we get all of that out of a program?

Muse now has guitar, piano, bass, voice, trumpet, saxophone and a drum kit. The built-in sounds discussed here are generated using mathematics. There are no recordings of guitars or singers hidden inside these instruments.

The techniques come from established ideas in acoustics and audio synthesis. Muse combines them with its own small recipes: a little more brightness here, a shorter decay there, a touch of breath before a note. The references below explain those underlying ideas; the exact settings describe this implementation.

Let’s meet the band.

## A note is a small crowd of sounds

Imagine playing middle C on a piano.

We tend to think of it as one sound at one frequency. In practice, several vibrations arrive together. There is a main frequency that helps us recognise the pitch, accompanied by higher-frequency components that give the note its character.

These components are called *partials*. When their frequencies are exact whole-number multiples of the main frequency, they are called *harmonics*.

A useful way to think about them is as ingredients. Two instruments can begin with the same basic pitch but use different amounts of the other ingredients. They can also change those amounts as the note continues.

This is the idea behind **additive synthesis**: make several simple waves and add them together. Julius O. Smith explains the method in [Spectral Audio Signal Processing](https://dsprelated.com/freebooks/sasp/Spectrum_Analysis_Sinusoids.html).

Our earlier approach already combined waves. The new instrument models give each component more independence.

That matters because a guitar note does more than become quieter. It also becomes less bright. The high-frequency components fade first, leaving the lower ones behind. The UNSW Acoustics Group shows this behaviour in its explanation of [vibrating strings](https://newt.phys.unsw.edu.au/jw/strings.html).

So our recipe becomes:

1. Generate several waves.
2. Give each one a starting strength.
3. Let each one fade at its own speed.
4. Add everything together.

The Go interface is pleasantly small:

```go
type instrument = func(freq, duration float64) []float64
```

The equals sign makes instrument a Go type alias: a shorter name for this particular kind of function. Any function with these inputs and output can serve as one of our instruments.

The two inputs, freq and duration, are decimal numbers. The first is the frequency in hertz; the second is the length in seconds. Passing 440 and 1 asks for a one-second note around concert A.

The result, []float64, is a Go slice—a sequence of decimal numbers. Each number is one audio sample, describing the wave at one instant. At Muse’s sample rate, a one-second note needs 44,100 of them. Positive and negative values describe opposite directions of the wave; a run of zeros represents silence.

The built-in pitched instruments scale their output to a peak magnitude of one. Muse applies the score’s volume later, so the instrument function can concentrate on the shape of the sound.

That is the contract: give us a pitch and a duration, and hand back a wave. The interesting part is deciding what those numbers should be.

## The guitar: where you pluck matters

Pluck a guitar string near the bridge, then nearer the middle. The pitch stays the same, but the colour changes.

The plucking position changes which vibrations are strongly excited and which are suppressed. Neville Fletcher explains this relationship in [Plucked Strings — A Review](https://www.phys.unsw.edu.au/music/people/publications/Fletcher1976d.pdf).

Muse captures part of that behaviour with a small recipe:

**A(n) = sin(n × π × p) / n**

Here, A(n) is the starting strength of the nth partial, and p is the plucking position along the string.

Don’t let the symbols spoil the music. We are asking two simple questions: which component are we making, and how much of it should we add?

Muse sets p to 0.2, representing a pluck one fifth of the way along the string. In the code, the equation becomes:

```go
amp := math.Sin(n*math.Pi*pluckAt) / n
```

When n is five, ten, fifteen and so on, the sine term is effectively zero. Every fifth component disappears from the mixture. Dividing by n also makes the higher components generally weaker.

This is a simplified sound recipe, rather than a complete physical model of a guitar.

Here is the instrument function from Muse, with the partial construction spread over a few lines for readability:

```go
func guitar(freq, duration float64) []float64 {
    const (
        pluckAt = 0.2
        stiff   = 0.0001
    )

    var ps []partial
    for n := 1.0; n <= 24; n++ {
        f := n * freq * math.Sqrt(1+stiff*n*n)
        amp := math.Sin(n*math.Pi*pluckAt) / n
        ps = append(ps, partial{
            f,
            amp,
            [][2]float64{{1, loss(f, 0.5, 2, 2)}},
        })
    }

    return additive(sampleCount(duration), ps, 0.0015, 0.01)
}
```

Let’s walk through it. This function first prepares the ingredients; the final call does the actual mixing.

The constants pluckAt and stiff describe our imaginary string. The first says where it is plucked. The second controls how much its frequencies depart from the tidy multiples of an ideal string.

The line var ps []partial creates an initially empty list of components. Each trip through the loop adds one more with append. The loop runs from one to 24, so we prepare up to 24 components. Starting n at 1.0 makes it a floating-point number, which fits the calculations using math.Sin and math.Sqrt.

The line beginning f := chooses the component’s frequency. Without the square-root factor, n * freq would give us the main frequency, twice that frequency, three times it, and so on. The stiff setting nudges those frequencies slightly sharp, increasingly so for the higher components. This follows the stiff-string relationship described in Smith’s [discussion of piano-string inharmonicity](https://dsprelated.com/freebooks/pasp/Piano_Synthesis.html).

The next line, amp :=, is our plucking-position equation in Go. It decides how much of this component goes into the mixture. Occasionally amp is negative. That turns the component’s wave upside down; it doesn’t mean “negative loudness”.

The partial value packages three things, in order: frequency, starting strength and decay settings. The most intimidating-looking bit, [][2]float64, simply means “a list of pairs of decimal numbers”. Each pair contains a weight and a decay rate. Our guitar uses one pair: the weight is 1, and loss calculates how quickly that component should fade.

The loss function gives higher frequencies larger decay rates, so they disappear sooner. The numbers 0.5, 2 and 2 are the coefficients of that recipe. They are settings we can experiment with, rather than a secret guitar password.

Finally, sampleCount(duration) converts seconds into a number of samples. The additive function takes that count and our prepared list, generates the waves and adds them together. Its last two arguments specify a 1.5-millisecond fade-in and a 10-millisecond fade-out, shortened if the note is very brief. Those little ramps avoid abrupt boundaries that can produce clicks.

That gives us the shape of a guitar note: a bright pluck followed by a gentler ring. The upper components leave the party early; the lower ones linger.

### A small shortcut inside the renderer

Muse produces 44,100 samples for every second of audio. Calculating a fresh sine and exponential for every component of every sample would repeat a lot of work.

For a steady pitch, each sample advances the wave by the same amount. The renderer remembers its position and rotates it forward, using additions and multiplications. Smith describes this as a [planar rotation oscillator](https://dsprelated.com/freebooks/pasp/Matlab_Sinusoidal_Oscillator_Implementations.html).

Fading works similarly: multiply the strength by the same slightly-smaller-than-one number at each step. Over many steps, the sound fades smoothly.

The renderer also leaves out partials that would be too high for the sample rate. Otherwise, they could fold back into the audible range as unwanted frequencies. More ingredients are not always better.

## The piano and bass: same machinery, different recipes

The piano uses the same basic machinery as the guitar, but changes the ingredients.

### A piano note begins with a hammer

A piano hammer strikes the string rather than plucking it. Its felt also affects how much high-frequency energy enters the sound.

Muse approximates this by softening the higher partials. It gives them a quick initial decay followed by a longer, quieter tail.

For most of its piano range, it also generates two strings with a very small tuning difference.

Why deliberately make them disagree?

Because nearby frequencies move into and out of alignment. Sometimes their waves reinforce each other; sometimes they partially cancel. The combined sound gently grows and shrinks. This is called *beating*.

You may have heard it while tuning an instrument: two almost-matching notes produce a slow “wah-wah” effect. As they get closer, that movement slows.

Muse uses a tiny amount of this to keep the piano tone from being perfectly still.

A complete piano simulation would have much more to consider, including the hammer, bridge and soundboard. Smith’s [piano synthesis chapter](https://dsprelated.com/freebooks/pasp/Piano_Synthesis.html) describes those larger models. Muse takes a smaller selection of ideas and keeps the implementation manageable.

### The bass knows its place

The bass also begins with a pluck, but its recipe strongly reduces the upper-frequency components.

We keep a little brightness at the beginning so the note has a clear attack. That brightness fades quickly, leaving a steadier low sound underneath the arrangement.

The bass doesn’t need to compete with the singer for every bit of attention. It has a different job.

The exact decay times, tuning offsets and brightness settings in these instruments are Muse’s design choices. The underlying techniques are established, but there is no universal set of numbers that means “guitar” or “piano”.

That is where synthesis becomes sound design.

## Giving the computer a voice

The built-in voice sings “ah”.

It won’t remember the lyrics, but it also won’t get them wrong.

To understand how it works, try holding a note while changing from “ah” to “ee”. The pitch can stay roughly the same, yet the sound changes dramatically.

Your vocal folds provide the vibration. Your throat and mouth shape it, strengthening some frequency regions more than others. These resonances help us recognise vowels.

This is the **source-filter model** of the voice, explained by the UNSW Acoustics Group in its [introduction to voice acoustics](https://www.phys.unsw.edu.au/jw/voice.html).

Think of the source as a buzzy musical signal and the filter as a set of tone controls.

Muse creates the buzz from a collection of harmonics. It then emphasises three frequency regions to suggest an “ah” vowel. Their centres match the first three frequencies in the Csound manual’s [tenor “a” table](https://csound.com/manual/misc/formants/#table-d21-tenor-a), although Muse uses its own strengths and widths.

We are shaping the sound directly. There is no simulated tongue moving around inside the program.

### A little movement goes a long way

A perfectly steady synthetic vowel can sound rather lifeless, so Muse gives it some movement.

The pitch drifts slightly. The loudness fluctuates a little. A brief touch of noise suggests breath at the beginning.

Longer notes develop *vibrato*: a small, regular movement above and below the target pitch.

Muse waits before introducing it, then gradually increases its depth. Short notes therefore remain relatively straight, while held notes acquire movement.

These are settings for this particular synthetic voice. They aren’t rules that every human singer follows.

There is also an important programming detail. When the pitch changes, we must keep the wave moving smoothly. Restarting or jumping around the wave would introduce unwanted sounds.

Muse keeps track of its current position in the wave and changes how quickly that position advances. It is like turning a wheel slightly faster or slower without suddenly moving it to another position.

That gives us a continuous tone while the pitch bends. Trigonometric identities then let the renderer derive the higher harmonics from the main wave, avoiding a separate sine calculation for every one.

## Trumpet and saxophone: louder also means brighter

The trumpet and saxophone share a sound-generating function. They differ in their settings: how quickly they begin, which frequencies they emphasise, how much breath noise they contain, and how the pitch moves.

One particularly useful idea is that a wind instrument changes colour as it gets louder.

On brass instruments, stronger playing can make the upper harmonics grow much more than the lower ones. The result becomes brighter and more forceful. The UNSW Acoustics Group demonstrates this with recordings and spectra in its [brass acoustics explanation](https://www.phys.unsw.edu.au/jw/brassacoustics.html).

Muse approximates that behaviour during the attack.

At first, the lower components dominate. As the simulated breath builds, more of the upper components emerge. The sound opens up.

The trumpet gets a firmer beginning and a brighter mixture. The saxophone gets a softer attack, more breath noise and stronger vibrato.

Both also begin slightly below the target pitch and slide upwards into it. The trumpet gets there quickly; the saxophone takes a little longer.

None of these details is enough by itself. Put them together, though, and the two instruments begin to develop different personalities.

## The drummer arrives

A drum kit needs another set of tricks.

Many drum sounds contain a mixture of a short pitched sound and noise. Cymbals can contain lots of frequencies that don’t line up into a neat harmonic family.

This is familiar territory in electronic synthesis. Gordon Reid’s Synth Secrets articles explain practical approaches to [bass drums](https://www.soundonsound.com/techniques/synthesizing-drums-bass-drum) and [snare drums](https://www.soundonsound.com/techniques/practical-snare-drum-synthesis). Muse uses the same broad building blocks, with its own recipes.

### The kick: a tiny falling note

A kick drum might sound like a simple thump, but its beginning contains a lot of information.

Muse starts with a sine wave whose pitch falls rapidly:

**f(t) = 48 + 100 × exp(-t / 0.035)**

Here, t is elapsed time in seconds, and f(t) is the frequency at that moment. The exp part is an exponential: in this expression, it starts at one and shrinks towards zero.

At the beginning, the frequency is therefore 148 Hz. As time passes, it approaches a low 48 Hz tone.

The number 0.035 controls how quickly the drop happens. After 35 milliseconds, the frequency is already down to about 85 Hz. The movement is too quick to sound like a leisurely slide down a keyboard. We hear it as part of the impact.

Here is the corresponding Go code, with the final expression split into named pieces for readability:

```go
func kick(rng *rand.Rand, n int) []float64 {
    x := make([]float64, n)
    phase := 0.0

    for k := range x {
        t := float64(k) / sampleRate

        // Drop quickly from 148 Hz towards 48 Hz.
        f := 48 + 100*math.Exp(-t/0.035)
        phase += 2 * math.Pi * f / sampleRate

        // The low body of the drum fades relatively slowly.
        body := math.Sin(phase) * math.Exp(-t/0.28)

        // A brief noise burst supplies the beater's click.
        noise := rng.Float64()*2 - 1
        click := 0.3 * noise * math.Exp(-t/0.003)

        x[k] = body + click
    }

    return x
}
```

This function builds the sound one sample at a time. Unlike the guitar’s loop, which prepares a list of ingredients, this loop writes directly into the finished sound buffer.

The argument n is the number of samples we want. The line make([]float64, n) allocates that many slots, initially filled with zeros. The other argument, rng, supplies pseudo-random numbers for the noisy click. Passing the generator in lets the caller choose a repeatable seed.

In for k := range x, k is the current sample’s position: zero, one, two, and so on. Dividing it by sampleRate turns that position into elapsed seconds. For example, sample 441 is ten milliseconds into a sound recorded at 44,100 samples per second. The float64 conversion keeps this as a decimal calculation.

The line beginning f := is our falling-pitch equation. math.Exp performs the exponential calculation, giving a shrinking contribution on top of the 48 Hz base.

The phase variable remembers our position in the wave. A complete turn is 2 * math.Pi radians, and f tells us how many turns to make per second. Dividing by sampleRate gives the advance for one sample. The += adds that advance to our existing position, keeping the wave continuous as its speed changes.

Next, math.Sin(phase) gives us the wave’s value at that position. Multiplying it by math.Exp(-t/0.28) gradually turns it down. This is body, the low thump. Pitch and loudness have separate controls: the pitch drops quickly, while the body takes longer to fade.

For the click, rng.Float64() supplies a value between zero and one. Multiplying by two and subtracting one spreads it between minus one and one, giving us noise that moves in both directions. The 0.3 multiplier sets its starting strength. The much smaller decay setting, 0.003, makes it vanish quickly.

The line x[k] = body + click combines those two ingredients and stores the current sample. Once the loop has filled every slot, return x hands the waveform back to the caller. The drum-kit setup normalises it and fades its final edge before caching it for playback.

A buffer, a clock, a falling wave and a burst of noise: we have a kick.

Try increasing 0.035 to make the pitch fall more slowly. Increase 0.28 and the low sound hangs around longer. Reduce the click’s 0.3 multiplier and the attack becomes softer.

A few numbers, a little experimentation, and our drummer has a different bass drum. No trip to the music shop required.

### The snare: thump plus rattle

The snare combines two short, low tones with a brighter noise burst.

The tones provide the body. The noise suggests the wires rattling against the lower drum head.

If we kept only the noise, the sound would be thin. If we kept only the tones, we would lose much of what makes it recognisable as a snare.

A filter removes some of the low frequencies from the noise, allowing the body and rattle to occupy different parts of the sound.

That division is also central to Reid’s [practical snare synthesis example](https://www.soundonsound.com/techniques/practical-snare-drum-synthesis).

### Hi-hats and crash: noise with manners

The hi-hats begin as noise too.

Muse removes much of the lower-frequency content and gives the remaining bright sound a short decay. For an open hi-hat, it lets the sound continue longer.

The crash uses a similar idea with a longer wash.

These are economical approximations. Real cymbals have much more complicated vibrations, as Reid explains in [his discussion of metallic percussion](https://www.soundonsound.com/techniques/analysing-metallic-percussion).

Still, filtered noise gets us a useful part of the way.

### Ride, toms and the rest of the kit

The ride cymbal combines noise with several tones that don’t fit a simple harmonic series. Their slightly awkward relationship helps create a metallic sound.

The toms use the kick’s falling-pitch idea at higher pitches.

The clap is particularly fun: three quick noise bursts arrive a few milliseconds apart, followed by a short tail. Several hands are clapping, and they haven’t quite agreed on the exact instant.

The rim click is a short tone mixed with filtered noise.

The cowbell combines two buzzy tones, then filters and fades them. It is available whenever the arrangement needs more cowbell. This is a programming decision I don’t think requires further justification.

### Avoiding the photocopier effect

Repeatedly playing exactly the same hit can sound mechanical.

Muse generates four seeded versions of each drum and cycles through them. Sounds containing noise or random starting phases get small differences between versions. Fully deterministic recipes remain unchanged.

The seed makes the results repeatable. Rendering the same score again doesn’t send the drummer off on a new interpretation.

The drums also keep their natural tails. A crash struck on a short written note is allowed to ring underneath what follows.

## Filters: choosing which parts of a sound survive

We have used the word filter several times. What does one actually do?

An audio filter changes the balance of frequencies.

A **low-pass filter** reduces high-frequency content, making a sound darker. A **high-pass filter** reduces low-frequency content, making it thinner or brighter. A **band-pass filter** emphasises a region between the two.

They don’t need to inspect the entire sound at once. Small digital filters can calculate each new output using the current sample and a few remembered values.

You can think of a simple low-pass filter as being reluctant to follow sudden changes. Slow changes get through easily. Rapid changes are smoothed out.

Muse uses these small filters for breath noise, drum noise and the brightness of softly played instruments.

Its band-pass filter has a specific named source: **Robert Bristow-Johnson’s Audio EQ Cookbook**, now available as a [W3C technical note](https://www.w3.org/TR/audio-eq-cookbook/).

That cookbook provides the coefficient recipes—the numbers that tell the filter how to behave. Muse chooses a centre frequency and a width, calculates the coefficients, then runs the samples through the filter.

The filter design is established. Choosing where to place it for a clap or cowbell is part of the instrument recipe.

## Envelopes: how a sound enters and leaves

An envelope describes how a sound changes in strength.

Does it begin suddenly? Fade in slowly? Remain steady? Die away immediately?

A common envelope is **ADSR**: attack, decay, sustain and release. The [Csound documentation](https://csound.com/manual/opcodes/adsr/) describes this conventional four-stage shape.

Imagine pressing a synthesiser key:

- **Attack:** the sound rises.
- **Decay:** it settles from its initial peak.
- **Sustain:** it holds a level while the note continues.
- **Release:** it fades away at the end.

Muse also has pluck, bell, bowed, staccato and tremolo envelopes.

Pluck and bell decay according to elapsed time. Making the written note longer gives them more time to ring; it doesn’t automatically make their fading happen more slowly.

The bowed envelope starts gently and swells. Staccato deliberately stops sounding before the written duration ends.

Tremolo makes the amplitude pulse. Vibrato, which we used for the voice, makes the pitch move. Similar-sounding names, different jobs.

The older harmonic colours remain available too. These are different mixtures of sine waves that can be paired with envelopes. The hollow colour leaves out even-numbered harmonics, while bright includes both odd and even ones. Mellow weakens the higher components more strongly.

They provide a small sound-design playground alongside the named instruments.

## Teaching the band to play together

We now have individual sounds, but a performance needs more than a collection of instruments.

If every note has identical strength, begins at an exact mathematical instant and stops as soon as the next one starts, even good sounds can feel stiff.

Muse adds a few simple performance rules.

These rules are its own heuristics. They are not a scientific model of how every musician plays.

### Louder, softer and somewhere in between

Dynamic markings such as p, mf and f control the strength of successive notes.

Muse uses decibels to organise those levels. Decibels express ratios, which makes them useful for describing changes across a wide range of amplitudes. Øland and Dannenberg explain the background in [Loudness Concepts & Pan Laws](https://www.cs.cmu.edu/~music/icm-online/readings/panlaws/panlaws.pdf).

Muse’s particular spacing between markings is a design choice.

A crescendo gradually raises the levels assigned to successive notes. Softer pitched instrument notes also receive filtering that reduces their brightness.

The first note of a section receives a stronger accent, and a small seeded variation changes note strength. The melody gets a little phrase shaping too. Within a phrase, higher and longer notes can remain slightly fuller while others are reduced.

The adjustments are deliberately small. We are nudging the performance, not rewriting it.

### A few milliseconds of freedom

Humanisation shifts note starts slightly around their scheduled positions.

The crucial detail is that each shift is measured from the shared timeline. We don’t keep adding one delay to the next, which would eventually leave the bass in a different postcode from the melody.

Chords can also be rolled. Their notes begin from low to high with small gaps between them. Guitar gets a wider spread than piano, suggesting a strum.

Drum hits written together stay together.

### Swing and slowing down

Swing changes the balance between two halves of a beat. The first becomes longer and the second shorter.

At full swing, Muse uses a two-to-one relationship. Smaller settings produce a gentler effect.

Ritardando gradually stretches time within a section. All parts follow the same time map, so the singer, accompaniment and drummer slow down together.

A fermata adds a hold at the end. Pitched notes reaching the boundary can be extended; drum sounds keep their natural decay while later events wait.

The maths here is bookkeeping: decide how written time should stretch, then place every note using that same rule.

## Giving the band a room

The renderer places each note into its part’s audio buffer at the scheduled start. Where notes overlap, their samples are added. This allows strings and cymbals to ring underneath what follows.

The instruments also need positions.

Panning controls how much of each part reaches the left and right speakers. Muse uses an **equal-power** curve, which helps avoid an obvious level dip as a sound moves through the centre. The method is explained in [Øland and Dannenberg’s discussion of pan laws](https://www.cs.cmu.edu/~music/icm-online/readings/panlaws/panlaws.pdf).

Then comes reverberation.

Clap your hands in a room and the sound doesn’t simply stop. Reflections arrive from different surfaces, overlap, and gradually fade.

Muse’s reverb follows **Freeverb**, the design by Jezar at Dreampoint documented in [Smith’s Physical Audio Signal Processing](https://dsprelated.com/freebooks/pasp/Freeverb.html).

It uses several short delay loops. Each sends some sound around again, a little quieter and darker. Their different lengths make the echoes overlap. Further stages spread them into a denser tail, while small left-right differences provide width.

The code calls those later stages “allpass” filters, following Freeverb’s terminology. Technically, they are approximations at the setting used here—a distinction Smith explains in the same reference.

Finally, everything is added together and written to a stereo WAV file.

One practical caution remains: several well-behaved sounds can become too loud when combined. Chords, ringing notes and reverb all add energy. The mix still needs some room below the maximum level.

Even a software band can turn itself up too far.

## Let’s hear it

For the final example, here is **Bright Side**, an original arrangement in F major at 120 beats per minute.

The saxophone opens with a short hook. The synthetic voice takes over in the verse. Piano supplies the early accompaniment, guitar joins the chorus, and the bass keeps moving underneath.

The drum kit works through rim clicks, hi-hats, ride cymbal and tom fills. At the end, the band slows down and holds the final chord.

The project is available on [GitHub](https://github.com/sausheong/gomuse). With the current score in scores/bright_side.yaml, render it from the repository with:

```shell
go run ./cmd/muse scores/bright_side
```

Listen for the different beginnings of the sounds: the guitar’s pluck, the piano’s strike, the saxophone sliding into its note, and the voice gradually developing vibrato. Underneath them is our falling sine-wave kick and its noisy companions.

About a minute of music, assembled from numbers. The drummer even turns up on time.

[EMBED AUDIO HERE: Bright Side — 62.76 seconds. Caption: “Bright Side: synthesised voice, saxophone, piano, guitar, bass and drums, rendered by Muse.”]
