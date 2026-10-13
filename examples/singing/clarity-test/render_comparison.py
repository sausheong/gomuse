"""Audition consonant emphasis using the existing lower-register neural render."""
import json
from pathlib import Path
import wave
import numpy as np

root = Path(__file__).resolve().parents[3]
source = root / 'examples/singing/register-test'
out = Path(__file__).resolve().parent
request = json.loads((source / 'octave_lower.json').read_text())
diagnostics = json.loads((source / 'octave_lower_raw.wav.json').read_text())
with wave.open(str(source / 'octave_lower_raw.wav')) as wav:
    params = wav.getparams()
    assert params.nchannels == 1 and params.sampwidth == 2
    audio = np.frombuffer(wav.readframes(wav.getnframes()), dtype='<i2').astype(float) / 32768
sr = params.framerate
vowels = set('aa ae ah ao aw ax ay eh er ey ih iy ow oy uh uw'.split())
envelope = np.ones(len(audio))
for phrase, rendered in zip(request['phrases'], diagnostics['phrases']):
    offset = round((phrase['notes'][0]['start'] - .25) * sr)
    for phone, frames in zip(rendered['phonemes'], rendered['frames']):
        end = offset + frames * 512
        symbol = phone.split('/')[-1]
        if symbol not in vowels | {'SP', 'AP'}:
            # Less emphasis on sibilants; raise stops and other consonants more.
            db = 3 if symbol in {'s', 'z', 'sh', 'zh', 'ch', 'jh'} else 5
            envelope[max(0, offset):min(len(audio), end)] = 10 ** (db / 20)
        offset = end
# Smooth gain boundaries over 5 ms to prevent clicks; no timing or pitch edits.
width = round(.005 * sr) | 1
pad = width // 2
envelope = np.convolve(np.pad(envelope, (pad, pad), mode='edge'), np.ones(width)/width, mode='valid')
versions = [audio, audio * envelope]
window = slice(round(.3*sr), round(14.1*sr))
rms = [np.sqrt(np.mean(a[window]**2)) for a in versions]
gains = [min(rms)/r for r in rms]
peak = max(np.max(np.abs(a*g)) for a,g in zip(versions,gains))
gains = [g * min(1, .9/peak) for g in gains]
metrics = []
for name, samples, gain in zip(['current', 'clearer'], versions, gains):
    pcm = np.round(samples * gain * 32767).astype('<i2')
    with wave.open(str(out / (name+'.wav')), 'wb') as wav:
        wav.setparams(params)
        wav.writeframes(pcm.tobytes())
    metrics.append(dict(file=name+'.wav', seconds=len(pcm)/sr, sample_rate=sr, channels=1,
                        peak=float(np.max(np.abs(pcm.astype(float)))/32768),
                        clipped_samples=int(np.count_nonzero(np.abs(pcm.astype(float))>=32767))))
(out / 'metrics.json').write_text(json.dumps(metrics, indent=2)+'\n')
print(json.dumps(metrics))
