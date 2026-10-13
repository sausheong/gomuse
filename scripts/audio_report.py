#!/usr/bin/env python3
"""Measure PCM WAV files without third-party dependencies; report, never normalise."""
import argparse
import array
import hashlib
import json
import math
from pathlib import Path
import sys
import wave


def inspect(path):
    with wave.open(str(path), 'rb') as wav:
        if wav.getsampwidth() != 2 or wav.getcomptype() != 'NONE':
            raise ValueError(f'{path}: expected uncompressed 16-bit PCM')
        rate, channels, frames = wav.getframerate(), wav.getnchannels(), wav.getnframes()
        samples = array.array('h', wav.readframes(frames))
    if sys.byteorder != 'little':
        samples.byteswap()
    if not samples or len(samples) != frames * channels:
        raise ValueError(f'{path}: empty or incomplete audio')
    peak = max(abs(x) for x in samples)
    rms = math.sqrt(sum(float(x) * x for x in samples) / len(samples))
    db = lambda value: 20 * math.log10(value / 32768) if value else None
    return {'file': str(path), 'sha256': hashlib.sha256(path.read_bytes()).hexdigest(),
            'duration_seconds': frames / rate, 'channels': channels, 'sample_rate': rate,
            'peak_dbfs': db(peak), 'rms_dbfs': db(rms),
            'crest_db': 20 * math.log10(peak / rms) if rms else None,
            'dc_fraction': sum(samples) / len(samples) / 32768,
            'clipped_samples': sum(x in (-32768, 32767) for x in samples)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('paths', nargs='+', type=Path)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    paths = []
    for path in args.paths:
        paths.extend(sorted(path.glob('*.wav')) if path.is_dir() else [path])
    if not paths:
        parser.error('no WAV files found')
    records = [inspect(path) for path in paths]
    args.out.write_text(json.dumps(records, indent=2, allow_nan=False) + '\n')
    print(f'Measured {len(records)} WAV files; clipped samples: {sum(x["clipped_samples"] for x in records)}')
    if any(x['clipped_samples'] for x in records):
        sys.exit(1)

if __name__ == '__main__':
    main()
