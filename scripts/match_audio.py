#!/usr/bin/env python3
"""Create preserved, RMS-matched PCM comparisons with shared peak headroom."""
import argparse
import array
import hashlib
import json
import math
from pathlib import Path
import sys
import wave
from audio_report import inspect


def read(path):
    with wave.open(str(path), 'rb') as wav:
        if wav.getsampwidth() != 2 or wav.getcomptype() != 'NONE':
            raise ValueError('expected uncompressed 16-bit PCM')
        params = wav.getparams()
        data = array.array('h', wav.readframes(params.nframes))
    if sys.byteorder != 'little':
        data.byteswap()
    return params, data


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('before', type=Path)
    parser.add_argument('after', type=Path)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--start', type=float, default=0)
    parser.add_argument('--duration', type=float)
    parser.add_argument('--rms', type=float, default=-24)
    parser.add_argument('--peak', type=float, default=-1)
    args = parser.parse_args()
    values = [args.start, args.rms, args.peak] + ([] if args.duration is None else [args.duration])
    if not all(math.isfinite(x) for x in values) or args.start < 0 or (args.duration is not None and args.duration <= 0):
        parser.error('invalid time or level')
    if not -60 <= args.rms <= -6 or not -12 <= args.peak <= -.1:
        parser.error('RMS must be -60..-6 dBFS; peak -12..-0.1 dBFS')
    if args.out.exists():
        parser.error('output directory already exists')
    p, before = read(args.before)
    q, after = read(args.after)
    if (p.framerate, p.nchannels) != (q.framerate, q.nchannels):
        parser.error('sample rates and channel counts must match')
    start = round(args.start * p.framerate) * p.nchannels
    end = min(len(before), len(after))
    if args.duration is not None:
        end = min(end, start + round(args.duration * p.framerate) * p.nchannels)
    clips = [before[start:end], after[start:end]]
    if not clips[0]:
        parser.error('selected interval is empty')
    peak_limit = 32767 * 10 ** (args.peak / 20)
    target = 32767 * 10 ** (args.rms / 20)
    levels = []
    for clip in clips:
        fade = min(round(.01 * p.framerate), len(clip) // p.nchannels // 2)
        for i in range(fade):
            for ch in range(p.nchannels):
                a, b = i * p.nchannels + ch, len(clip) - (i + 1) * p.nchannels + ch
                clip[a] = round(clip[a] * i / fade)
                clip[b] = round(clip[b] * i / fade)
        rms = math.sqrt(sum(float(x) ** 2 for x in clip) / len(clip))
        peak = max(abs(x) for x in clip)
        if not rms:
            parser.error('cannot match a silent interval')
        levels.append(rms)
        target = min(target, peak_limit * rms / peak)
    args.out.mkdir(parents=True)
    results = []
    for label, source, clip, rms in zip(('before', 'after'), (args.before, args.after), clips, levels):
        gain = target / rms
        data = array.array('h', (round(x * gain) for x in clip))
        if sys.byteorder != 'little':
            data.byteswap()
        path = args.out / (label + '.wav')
        with wave.open(str(path), 'wb') as wav:
            wav.setparams(p)
            wav.writeframes(data.tobytes())
        report = inspect(path)
        report.update(source=str(source), source_sha256=hashlib.sha256(source.read_bytes()).hexdigest(), gain=gain)
        results.append(report)
    report = dict(start_seconds=start / p.nchannels / p.framerate, reports=results,
                  method='RMS matched with 10 ms edge fades; common target reduced if needed for peak headroom. Not perceptual LUFS.')
    (args.out / 'comparison.json').write_text(json.dumps(report, indent=2, allow_nan=False) + '\n')
    print(f'Matched to {20 * math.log10(target / 32768):.2f} dBFS RMS; output {args.out}')

if __name__ == '__main__':
    main()
