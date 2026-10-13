"""Create equal-length, whole-file RMS-matched comparison WAVs (stdlib only)."""
import array
import json
import math
from pathlib import Path
import wave

root = Path(__file__).resolve().parent
recordings = {}
for name in ("before", "after"):
    with wave.open(str(root / f"{name}.wav")) as source:
        params = source.getparams()
        if params.sampwidth != 2:
            raise ValueError("Expected 16-bit PCM")
        samples = array.array("h", source.readframes(source.getnframes()))
        import sys
        if sys.byteorder != "little":
            samples.byteswap()
        recordings[name] = (params, samples)

before_params = recordings["before"][0]
for params, _ in recordings.values():
    if params[:3] != before_params[:3]:
        raise ValueError("Audio formats must match")
length = max(len(samples) for _, samples in recordings.values())
metrics = {}
for name, (params, samples) in recordings.items():
    original_length = len(samples)
    peak = max(abs(x) for x in samples)
    clipped = sum(x >= 32767 or x <= -32768 for x in samples)
    samples.extend([0] * (length - len(samples)))
    rms = math.sqrt(sum(x*x for x in samples) / len(samples))
    gain = 32768 * 10**(-23/20) / rms
    scaled = [round(x*gain) for x in samples]
    if max(scaled) >= 32767 or min(scaled) <= -32768:
        raise ValueError("Comparison gain would clip")
    output = array.array("h", scaled)
    if sys.byteorder != "little":
        output.byteswap()
    with wave.open(str(root / f"{name}-matched.wav"), "wb") as destination:
        destination.setparams(params)
        destination.writeframes(output.tobytes())
    metrics[name] = {
        "duration_seconds": original_length / params.nchannels / params.framerate,
        "comparison_duration_seconds": length / params.nchannels / params.framerate,
        "channels": params.nchannels,
        "sample_rate": params.framerate,
        "peak_dbfs": 20*math.log10(peak/32768),
        "clipped_samples": clipped,
        "comparison_gain": gain,
        "matched_rms_dbfs": 20*math.log10(math.sqrt(sum(x*x for x in scaled)/length)/32768),
        "matched_peak_dbfs": 20*math.log10(max(abs(x) for x in scaled)/32768),
        "matched_clipped_samples": 0,
    }
(root / "metrics.json").write_text(json.dumps(metrics, indent=2) + "\n")
print(json.dumps(metrics, indent=2))
