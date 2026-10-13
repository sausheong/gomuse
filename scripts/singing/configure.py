#!/usr/bin/env python3
"""Create a local Muse singer config for an installed DiffSinger ONNX voicebank."""
import argparse
import hashlib
import json
import math
import sys
from pathlib import Path

import yaml


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--voicebank", type=Path, required=True)
    p.add_argument("--vocoder", type=Path, required=True, help="Matching vocoder.yaml")
    p.add_argument("--output", type=Path, default=Path(".local/singing/singer.json"))
    p.add_argument("--speaker", default="")
    p.add_argument("--controls", default="{}", help="Explicit model variance controls as JSON")
    p.add_argument("--prosody", choices=("auto", "neural", "score"), default="auto")
    p.add_argument("--expression", type=float, default=1.0, help="Neural pitch expression, 0..1")
    args = p.parse_args()
    if not math.isfinite(args.expression) or not 0 <= args.expression <= 1:
        p.error("expression must be 0..1")
    bank = args.voicebank.resolve()
    vocoder = args.vocoder.resolve()
    bank_config = bank / "dsconfig.yaml"
    if not bank_config.is_file() or not vocoder.is_file():
        p.error("Install the voicebank and matching vocoder first; dsconfig.yaml and vocoder.yaml must exist")
    cfg = yaml.safe_load(bank_config.read_text(encoding="utf-8-sig"))
    voc = yaml.safe_load(vocoder.read_text(encoding="utf-8-sig"))
    for key in ("acoustic", "phonemes"):
        if key not in cfg or not (bank / cfg[key]).is_file():
            p.error(f"Voicebank is missing {key}")
    if "model" not in voc or not (vocoder.parent / voc["model"]).is_file():
        p.error("Vocoder is missing its ONNX model")
    try:
        controls = json.loads(args.controls)
        if not isinstance(controls, dict):
            raise ValueError("controls must be an object")
    except ValueError as e:
        p.error(str(e))
    backend = Path(__file__).resolve().with_name("diffsinger_onnx.py")
    # Fingerprint the actual local model files, adapter and runtime versions so
    # reconfiguration after an update invalidates all old cached phrase audio.
    files = {backend, bank_config, vocoder, vocoder.parent / voc["model"]}
    files.update(f for f in bank.rglob("*") if f.suffix.lower() in (".onnx", ".emb", ".yaml", ".json", ".txt"))
    digest = hashlib.sha256()
    for path in sorted(files):
        digest.update(str(path).encode())
        with path.open("rb") as f:
            for chunk in iter(lambda: f.read(1024 * 1024), b""):
                digest.update(chunk)
    import importlib.metadata
    for package in ("onnxruntime", "numpy", "cmudict", "PyYAML"):
        digest.update(f"{package}={importlib.metadata.version(package)}".encode())
    command = [sys.executable, str(backend), "--voicebank", str(bank), "--vocoder", str(vocoder), "--controls", json.dumps(controls)]
    command += ["--prosody", args.prosody, "--expression", str(args.expression)]
    if args.speaker:
        command += ["--speaker", args.speaker]
    config = {"command": command, "timeout_seconds": 300, "cache_dir": "cache", "revision": digest.hexdigest()}
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(config, indent=2) + "\n")
    print(f"Wrote {args.output}; model compatibility and singing quality still require a real render.")


if __name__ == "__main__":
    main()
