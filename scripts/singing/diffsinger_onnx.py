#!/usr/bin/env python3
"""Experimental local DiffSinger ONNX phrase renderer for Muse protocol v1.

Uses an OpenUtau-style voicebank and a separately installed matching vocoder.
No network access, automatic downloads, or fallback to speech/formant synthesis.
"""
import argparse
import json
import math
import re
import sys
import wave
from pathlib import Path

import cmudict
import numpy as np
import onnxruntime as ort
import yaml

VOWELS = set("aa ae ah ao aw ay eh er ey ih iy ow oy uh uw ax".split())


def read_yaml(path):
    with open(path, encoding="utf-8-sig") as f:
        return yaml.safe_load(f)


def phone_table(path):
    with open(path, encoding="utf-8-sig") as f:
        if path.suffix == ".json":
            data = json.load(f)
            return data if isinstance(data, dict) else {p: i for i, p in enumerate(data)}
        return {p.strip(): i for i, p in enumerate(f) if p.strip()}


def pronunciation(lyric, dictionary):
    if lyric.startswith("[") and lyric.endswith("]"):
        phones = lyric[1:-1].split()
    else:
        entries = dictionary.get(lyric.lower())
        if not entries:
            raise ValueError(f"Unknown word {lyric!r}; supply phonemes, e.g. '[s t ey]'")
        phones = [re.sub(r"[012]$", "", p).lower() for p in entries[0]]
    vowels = [i for i, p in enumerate(phones) if p.split("/")[-1] in VOWELS]
    if len(vowels) != 1:
        raise ValueError(f"Each lyric must be one syllable; {lyric!r} has {len(vowels)} vowels. Split it into syllables or explicit phonemes.")
    return phones, vowels[0]


def prepare_phrase(notes, table, timestep, dictionary, language="en"):
    """Allocate consonants around held vowels; '+' continues the same vowel.

    The first vowel starts on its note. Consonants anticipate later note onsets.
    This first version uses explicit timing heuristics, not a duration predictor.
    """
    lead = 0.25
    origin = notes[0]["start"] - lead
    finish = notes[-1]["start"] + notes[-1]["duration"] + lead
    frames = int(math.ceil((finish - origin) / timestep))
    groups = []
    for note in notes:
        if note["lyric"] == "+":
            if not groups:
                raise ValueError("A vowel extension needs a preceding syllable")
            groups[-1]["notes"].append(note)
        else:
            phones, vowel = pronunciation(note["lyric"], dictionary)
            groups.append({"notes": [note], "phones": phones, "vowel": vowel})

    def resolve(p):
        for key in (f"{language}/{p}", p):
            if key in table:
                return key
        raise ValueError(f"Voicebank has no phoneme {p!r}; provide a voicebank-specific phoneme override")

    spans = []
    cursor = 0.0
    for gi, group in enumerate(groups):
        ns, phones, vi = group["notes"], group["phones"], group["vowel"]
        start = ns[0]["start"] - origin
        end = ns[-1]["start"] + ns[-1]["duration"] - origin
        # Reserve room before the next vowel for its onset consonants.
        if gi + 1 < len(groups):
            following = groups[gi + 1]
            next_start = following["notes"][0]["start"] - origin
            end = min(end, next_start - min(0.06 * following["vowel"], 0.2 * (end - start)))
        consonant = min(0.06, (end - start) / max(4, len(phones) * 2))
        onset = max(cursor, start - vi * consonant)
        if onset > cursor:
            spans.append(("SP", cursor, onset))
        cursor = onset
        for pi, p in enumerate(phones):
            if pi < vi:
                stop = max(cursor + timestep, start - (vi - pi - 1) * consonant)
            elif pi == vi:
                stop = end - (len(phones) - vi - 1) * consonant
            else:
                stop = cursor + consonant
            if stop <= cursor:
                raise ValueError("Syllable too short for its consonants; lengthen the note")
            spans.append((resolve(p), cursor, stop))
            cursor = stop
    spans.append(("SP", cursor, frames * timestep))
    # Cumulative rounding preserves total length instead of rounding each phone.
    ids, durations, symbols = [], [], []
    prev = 0
    for p, _, end in spans:
        boundary = min(frames, int(math.floor(end / timestep + 0.5)))
        if boundary > prev:
            if p not in table:
                raise ValueError(f"Voicebank has no silence token {p!r}")
            ids.append(table[p]); durations.append(boundary - prev); symbols.append(p)
        prev = boundary
    if sum(durations) != frames:
        raise ValueError("Phoneme frame allocation does not cover the phrase")
    times = origin + np.arange(frames) * timestep
    f0 = np.zeros(frames, dtype=np.float32)
    # Keep pitch under pre-onset consonants; SP tokens carry silence.
    for i, note in enumerate(notes):
        start = note["start"] - (lead if i == 0 else 0)
        end = notes[i + 1]["start"] if i + 1 < len(notes) else finish
        hz = 440 * 2 ** ((note["midi"] - 69) / 12)
        f0[(times >= start) & (times < end)] = hz
    return origin, np.array(ids, dtype=np.int64)[None], np.array(durations, dtype=np.int64)[None], f0[None], symbols


def session(path, threads):
    opts = ort.SessionOptions()
    opts.intra_op_num_threads = threads
    opts.inter_op_num_threads = 1
    return ort.InferenceSession(str(path), sess_options=opts, providers=["CPUExecutionProvider"])


def check_inputs(sess, values):
    needed = {x.name for x in sess.get_inputs()}
    missing = needed - values.keys()
    if missing:
        raise ValueError(f"Unsupported model inputs: {sorted(missing)}. This voicebank needs additional conditioning; it cannot use this adapter yet.")
    return {k: values[k] for k in needed}


def language_ids(symbols, mapping):
    """Shared silence tokens use zero; prefixed phones use their language ID."""
    return np.array([[mapping[p.split("/", 1)[0]] if "/" in p else 0
                      for p in symbols]], dtype=np.int64)


def allocate_durations(predicted, word_div, word_dur, symbols=None, timestep=None):
    """Fit learned phone proportions to each written syllable, in whole frames."""
    predicted = np.asarray(predicted, dtype=np.float64).reshape(-1)
    if len(predicted) != sum(word_div) or not np.isfinite(predicted).all():
        raise ValueError("Invalid duration prediction")
    if symbols is not None and (len(symbols) != len(predicted) or timestep is None or timestep <= 0):
        raise ValueError("Phoneme timing needs matching symbols and a positive timestep")
    result, offset = [], 0
    for count, budget in zip(word_div, word_dur):
        if count < 1 or budget < count:
            raise ValueError("Syllable too short to allocate a frame to each phoneme")
        weights = np.maximum(predicted[offset:offset + count], 0)
        if weights.sum() <= 0:
            raise ValueError("Duration predictor returned no positive durations")
        # Reserve one frame per phone, then distribute the remaining frames.
        ideal = weights / weights.sum() * (budget - count)
        frames = np.floor(ideal).astype(np.int64) + 1
        remainder = int(budget - frames.sum())
        order = np.argsort(-(ideal - np.floor(ideal)), kind="stable")
        frames[order[:remainder]] += 1
        if symbols is not None:
            phones = [p.split("/")[-1] for p in symbols[offset:offset + count]]
            vowels = [i for i, p in enumerate(phones) if p in VOWELS]
            if len(vowels) == 1:
                vowel = vowels[0]
                # Stretch the sung vowel, never an onset or closing consonant.
                # Keep learned relative timing for short syllables, but bound
                # each consonant to 75 ms and each cluster to 120 ms.
                cap = max(1, int(0.075 / timestep))
                for i in range(count):
                    if i != vowel:
                        frames[i] = min(frames[i], cap)
                for indices in (list(range(vowel)), list(range(vowel + 1, count))):
                    cluster_budget = max(len(indices), int(0.12 / timestep))
                    while indices and sum(frames[i] for i in indices) > cluster_budget:
                        i = max(indices, key=lambda i: frames[i])
                        frames[i] -= 1
                consonants = [i for i in range(count) if i != vowel]
                # Also reserve at least 60% of short syllables for the vowel,
                # except when one-frame consonants make that impossible.
                consonant_budget = max(len(consonants), int(budget * 0.4))
                while consonants and sum(frames[i] for i in consonants) > consonant_budget:
                    i = max(consonants, key=lambda i: frames[i])
                    frames[i] -= 1
                frames[vowel] = budget - sum(frames[i] for i in consonants)
            elif phones not in (["SP"], ["AP"]):
                raise ValueError("Each sung syllable needs exactly one vowel")
        result.extend(frames.tolist())
        offset += count
    return np.array([result], dtype=np.int64)


def prosody_layout(notes, table, timestep, dictionary):
    """Keep syllable and note clocks separate, so '+' changes pitch, not phones."""
    origin = notes[0]["start"] - 0.25
    end = notes[-1]["start"] + notes[-1]["duration"]
    total = int(math.ceil((end + 0.25 - origin) / timestep))
    def frame(t): return int(math.floor((t - origin) / timestep + 0.5))
    groups = []
    for note in notes:
        if note["lyric"] == "+":
            if not groups:
                raise ValueError("A vowel extension needs a preceding syllable")
            groups[-1].append(note)
        else:
            groups.append([note])
    symbols, divisions, budgets, midi = ["SP"], [1], [frame(notes[0]["start"])], [notes[0]["midi"]]
    for group in groups:
        phones, _ = pronunciation(group[0]["lyric"], dictionary)
        resolved = []
        for p in phones:
            key = next((k for k in (f"en/{p}", p) if k in table), None)
            if key is None:
                raise ValueError(f"Voicebank has no phoneme {p!r}")
            resolved.append(key)
        symbols.extend(resolved)
        divisions.append(len(resolved))
        budgets.append(frame(group[-1]["start"] + group[-1]["duration"]) - frame(group[0]["start"]))
        mean_midi = sum(n["midi"] * n["duration"] for n in group) / sum(n["duration"] for n in group)
        midi.extend([int(round(mean_midi))] * len(resolved))
    symbols.append("SP"); divisions.append(1); budgets.append(total - frame(end)); midi.append(notes[-1]["midi"])
    note_bounds = [0, frame(notes[0]["start"])] + [frame(n["start"] + n["duration"]) for n in notes] + [total]
    note_dur = np.diff(note_bounds).astype(np.int64)[None]
    if np.any(note_dur <= 0) or sum(budgets) != total:
        raise ValueError("Neural prosody requires contiguous notes with positive frame durations")
    note_midi = np.array([[notes[0]["midi"]] + [n["midi"] for n in notes] + [notes[-1]["midi"]]], dtype=np.float32)
    note_rest = np.zeros_like(note_midi, dtype=np.bool_)
    note_rest[0, [0, -1]] = True
    return origin, symbols, divisions, budgets, np.array([midi], dtype=np.int64), note_midi, note_dur, note_rest


class ProsodyPredictor:
    """Duration -> expressive MIDI pitch -> Hz, using the published bank models."""

    def __init__(self, bank, acoustic_cfg, args):
        self.parts = {}
        self.languages = {}
        self.steps = args.steps
        self.expression = args.expression
        self.timestep = acoustic_cfg.get("hop_size", 512) / acoustic_cfg.get("sample_rate", 44100)
        for kind, folder in (("dur", "dsdur"), ("pitch", "dspitch")):
            root = bank / folder
            cfg = read_yaml(root / "dsconfig.yaml")
            if cfg.get("languages"):
                self.languages[kind] = json.loads((root / cfg["languages"]).read_text())
            for key, default in (("sample_rate", 44100), ("hop_size", 512)):
                if cfg.get(key, default) != acoustic_cfg.get(key, default):
                    raise ValueError(f"{kind}/acoustic {key} mismatch")
            emb = None
            speakers = cfg.get("speakers", [])
            if speakers:
                chosen = Path(args.speaker or acoustic_cfg["speakers"][0]).name
                matches = [s for s in speakers if Path(s).name == chosen]
                # Nishiren's duration and pitch predictors each have one shared voice.
                if not matches and len(speakers) == 1:
                    matches = speakers
                if len(matches) != 1:
                    raise ValueError(f"No matching {kind} speaker {chosen!r}")
                emb = np.fromfile(root / (matches[0] + ".emb"), dtype="<f4")
            self.parts[kind] = (phone_table(root / cfg["phonemes"]),
                                session(root / cfg["linguistic"], args.threads),
                                session(root / cfg[kind], args.threads), emb)

    def prepare(self, notes, table, dictionary):
        origin, symbols, divisions, budgets, ph_midi, note_midi, note_dur, note_rest = prosody_layout(notes, table, self.timestep, dictionary)
        dt, encoder, predictor, emb = self.parts["dur"]
        values = {"tokens": np.array([[dt[p] for p in symbols]], dtype=np.int64),
                  "word_div": np.array([divisions], dtype=np.int64),
                  "word_dur": np.array([budgets], dtype=np.int64)}
        if "dur" in self.languages:
            values["languages"] = language_ids(symbols, self.languages["dur"])
        encoded, masks = encoder.run(None, check_inputs(encoder, values))
        values = {"encoder_out": encoded, "x_masks": masks, "ph_midi": ph_midi}
        if emb is not None:
            values["spk_embed"] = np.broadcast_to(emb, (1, len(symbols), len(emb))).copy()
        raw_dur = predictor.run(None, check_inputs(predictor, values))[0]
        durations = allocate_durations(raw_dur, divisions, budgets, symbols, self.timestep)
        pt, encoder, predictor, emb = self.parts["pitch"]
        values = {"tokens": np.array([[pt[p] for p in symbols]], dtype=np.int64), "ph_dur": durations}
        if "pitch" in self.languages:
            values["languages"] = language_ids(symbols, self.languages["pitch"])
        encoded = encoder.run(None, check_inputs(encoder, values))[0]
        base = np.repeat(note_midi[0], note_dur[0]).astype(np.float32)
        # Match DiffSinger's 60 ms sine-window smoothing of the written MIDI curve.
        width = max(3, round(0.06 / self.timestep))
        kernel = np.sin(np.linspace(0, np.pi, width)); kernel /= kernel.sum()
        base = np.convolve(np.pad(base, (width // 2, (width - 1) // 2), mode="edge"), kernel, mode="valid").astype(np.float32)[None]
        values = {"encoder_out": encoded, "ph_dur": durations,
                  "note_midi": note_midi, "note_dur": note_dur, "note_rest": note_rest,
                  "pitch": base, "expr": np.full_like(base, self.expression),
                  "retake": np.ones_like(base, dtype=np.bool_), "steps": np.array(self.steps, dtype=np.int64)}
        if emb is not None:
            values["spk_embed"] = np.broadcast_to(emb, (*base.shape, len(emb))).copy()
        pitch = predictor.run(None, check_inputs(predictor, values))[0]
        if pitch.shape != base.shape or not np.isfinite(pitch).all() or np.any((pitch < 0) | (pitch > 127)):
            raise ValueError("Pitch predictor returned an invalid MIDI curve")
        f0 = (440 * np.exp2((pitch - 69) / 12)).astype(np.float32)
        tokens = np.array([[table[p] for p in symbols]], dtype=np.int64)
        detail = {"word_div": divisions, "word_frames": budgets,
                  "note_frames": note_dur[0].tolist(), "pitch_midi": pitch[0].tolist()}
        return origin, tokens, durations, f0, symbols, detail


class VariancePredictor:
    """Use a bank's duration-conditioned ONNX expression predictor unchanged."""

    def __init__(self, bank, acoustic_cfg, args):
        self.root = bank / "dsvariance"
        cfg = read_yaml(self.root / "dsconfig.yaml")
        for key, default in (("sample_rate", 44100), ("hop_size", 512)):
            if cfg.get(key, default) != acoustic_cfg.get(key, default):
                raise ValueError(f"Variance/acoustic {key} mismatch")
        self.table = phone_table(self.root / cfg["phonemes"])
        self.encoder = session(self.root / cfg["linguistic"], args.threads)
        self.predictor = session(self.root / cfg["variance"], args.threads)
        self.steps = args.steps
        self.names = [n for n in ("energy", "breathiness", "voicing", "tension")
                      if cfg.get("predict_" + n, False)]
        self.embedding = None
        if cfg.get("speakers"):
            selected = Path(args.speaker or acoustic_cfg["speakers"][0]).name
            matches = [s for s in cfg["speakers"] if Path(s).name == selected]
            if len(matches) != 1:
                raise ValueError(f"Variance model has no matching speaker {selected!r}")
            self.embedding = np.fromfile(self.root / (matches[0] + ".emb"), dtype="<f4")

    def predict(self, symbols, durations, f0):
        tokens = np.array([[self.table[p] for p in symbols]], dtype=np.int64)
        # This path supports encoders trained with supplied phoneme durations.
        # Encoders requiring word divisions or language IDs fail explicitly.
        encoded = self.encoder.run(None, check_inputs(self.encoder, {
            "tokens": tokens, "ph_dur": durations,
        }))[0]
        pitch = (69 + 12 * np.log2(np.maximum(f0, 1) / 440)).astype(np.float32)
        values = {"encoder_out": encoded, "ph_dur": durations, "pitch": pitch,
                  "steps": np.array(self.steps, dtype=np.int64),
                  "speedup": np.array(max(1, round(1000 / self.steps)), dtype=np.int64),
                  "retake": np.ones((*f0.shape, len(self.names)), dtype=np.bool_)}
        for name in self.names:
            values[name] = np.zeros_like(f0)  # All frames are marked for prediction.
        if self.embedding is not None:
            values["spk_embed"] = np.broadcast_to(
                self.embedding, (*f0.shape, len(self.embedding))).copy()
        results = self.predictor.run(None, check_inputs(self.predictor, values))
        outputs = {o.name.removesuffix("_pred"): v
                   for o, v in zip(self.predictor.get_outputs(), results)}
        for name in self.names:
            if name not in outputs or outputs[name].shape != f0.shape or not np.isfinite(outputs[name]).all():
                raise ValueError(f"Invalid predicted {name} curve")
        return {name: outputs[name] for name in self.names}


def render(args, request):
    if request["version"] != 1 or request["sample_rate"] != 44100 or request["language"] != "en":
        raise ValueError("Expected Muse protocol v1, 44.1 kHz, English")
    # Random ONNX operators initialise when sessions are created.
    ort.set_seed(args.seed)
    bank = Path(args.voicebank).resolve()
    cfg = read_yaml(bank / "dsconfig.yaml")
    vocpath = Path(args.vocoder).resolve()
    voc = read_yaml(vocpath)
    for key, default in (("sample_rate", 44100), ("hop_size", 512), ("num_mel_bins", 128)):
        if cfg.get(key, default) != voc.get(key, default):
            raise ValueError(f"Voicebank/vocoder {key} mismatch")
    rate = cfg.get("sample_rate", 44100)
    if rate != 44100:
        raise ValueError("This first adapter supports 44.1 kHz voicebanks only")
    hop = cfg.get("hop_size", 512)
    table = phone_table(bank / cfg["phonemes"])
    dictionary = cmudict.dict()
    acoustic = session(bank / cfg["acoustic"], args.threads)
    vocoder = session(vocpath.parent / voc["model"], args.threads)
    controls = json.loads(args.controls)
    needed_variances = {x.name for x in acoustic.get_inputs()} & {"energy", "tension", "breathiness", "voicing"}
    variance = None
    if needed_variances - controls.keys() and (bank / "dsvariance/dsconfig.yaml").is_file():
        variance = VariancePredictor(bank, cfg, args)

    has_prosody = all((bank / folder / "dsconfig.yaml").is_file() for folder in ("dsdur", "dspitch"))
    prosody = None
    if args.prosody == "neural" or (args.prosody == "auto" and has_prosody):
        prosody = ProsodyPredictor(bank, cfg, args)

    print("Acoustic inputs:", [x.name for x in acoustic.get_inputs()], file=sys.stderr)
    total = int(math.floor(request["duration"] * rate + 0.5))
    if total <= 0 or total > 600 * rate:
        raise ValueError("Duration must be positive and no more than 600 seconds")
    output = np.zeros(total, dtype=np.float64)
    diagnostics = []
    for i, phrase in enumerate(request["phrases"]):
        detail = {"prosody": "neural" if prosody else "score"}
        if prosody:
            origin, tokens, durations, f0, symbols, predicted = prosody.prepare(phrase["notes"], table, dictionary)
            detail.update(predicted)
        else:
            origin, tokens, durations, f0, symbols = prepare_phrase(phrase["notes"], table, hop / rate, dictionary)
        n = f0.shape[1]
        values = {
            "tokens": tokens, "durations": durations, "f0": f0,
            "gender": np.zeros((1, n), dtype=np.float32),
            "velocity": np.ones((1, n), dtype=np.float32),
            "steps": np.array(args.steps, dtype=np.int64),
            "speedup": np.array(max(1, round(1000 / args.steps)), dtype=np.int64),
            "depth": np.array(cfg.get("max_depth", 1.0), dtype=np.float32),
        }
        if cfg.get("languages"):
            languages = json.loads((bank / cfg["languages"]).read_text())
            values["languages"] = language_ids(symbols, languages)
        # Variance values must be supplied explicitly; arbitrary zeros can produce
        # invalid conditioning for models trained on logarithmic energy/noise.
        if variance is not None:
            values.update(variance.predict(symbols, durations, f0))
        for name, value in controls.items():
            if name not in ("tension", "breathiness", "voicing", "energy") or not math.isfinite(value):
                raise ValueError(f"Unsupported control {name!r}")
            values[name] = np.full((1, n), value, dtype=np.float32)
        speakers = cfg.get("speakers", [])
        if speakers:
            speaker = args.speaker or speakers[0]
            if speaker not in speakers:
                raise ValueError(f"Unknown speaker {speaker!r}; choose from {speakers}")
            emb = np.fromfile(bank / (speaker + ".emb"), dtype="<f4")
            values["spk_embed"] = np.broadcast_to(emb, (1, n, len(emb))).copy()
        print(f"Rendering phrase {i + 1}/{len(request['phrases'])} ({n * hop / rate:.2f}s)", file=sys.stderr, flush=True)
        mel = acoustic.run(None, check_inputs(acoustic, values))[0]
        # ONNX vocoders exist with both natural-log and base-10 mel inputs.
        source_base, target_base = str(cfg.get("mel_base", "e")), str(voc.get("mel_base", "e"))
        if source_base not in ("e", "10") or target_base not in ("e", "10"):
            raise ValueError("Only e and 10 mel logarithm bases are supported")
        if source_base != target_base:
            mel = mel * np.float32(math.log(10) if source_base == "10" else 1 / math.log(10))
        samples = vocoder.run(None, check_inputs(vocoder, {"mel": mel, "f0": f0}))[0].reshape(-1)
        if not np.isfinite(samples).all():
            raise ValueError("Neural renderer produced non-finite samples")
        if abs(len(samples) - n * hop) > hop:
            raise ValueError("Vocoder returned an unexpected duration")
        start = int(math.floor(origin * rate + 0.5))
        src = max(0, -start); dst = max(0, start)
        count = min(len(samples) - src, total - dst)
        if count > 0:
            output[dst:dst + count] += samples[src:src + count]
        diagnostics.append({"phrase": i + 1, "phonemes": symbols, "frames": durations[0].tolist(), **detail})
    peak = float(np.max(np.abs(output)))
    if peak < 1e-6:
        raise ValueError("Neural renderer produced silence")
    # One gain for the complete vocal, preserving phrase-to-phrase differences.
    output *= 0.85 / peak
    pcm = np.rint(output * 32767).astype("<i2")
    with wave.open(args.output, "wb") as f:
        f.setnchannels(1); f.setsampwidth(2); f.setframerate(rate); f.writeframes(pcm.tobytes())
    Path(args.output + ".json").write_text(json.dumps({"raw_peak": peak, "phrases": diagnostics}, indent=2))


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--voicebank", required=True)
    p.add_argument("--vocoder", required=True, help="Path to matching vocoder.yaml")
    p.add_argument("--speaker", default="")
    p.add_argument("--steps", type=int, default=20)
    p.add_argument("--threads", type=int, default=4)
    p.add_argument("--seed", type=int, default=42)
    p.add_argument("--controls", default="{}", help="Explicit constant variance conditioning as JSON")
    p.add_argument("--prosody", choices=("auto", "neural", "score"), default="auto", help="Use bundled pitch/duration models when present; score keeps the baseline")
    p.add_argument("--expression", type=float, default=1.0, help="Neural pitch expression, 0..1")
    p.add_argument("--request", required=True)
    p.add_argument("--output", required=True)
    args = p.parse_args()
    if not 1 <= args.steps <= 100 or not 1 <= args.threads <= 16:
        p.error("steps must be 1..100; threads must be 1..16")
    if not math.isfinite(args.expression) or not 0 <= args.expression <= 1:
        p.error("expression must be 0..1")
    try:
        render(args, json.loads(Path(args.request).read_text()))
    except Exception as e:
        print(f"DiffSinger: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
