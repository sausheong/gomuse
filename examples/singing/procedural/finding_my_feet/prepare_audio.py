"""Prepare freshly rendered WAVs, preserving relative phrase dynamics."""
from pathlib import Path
import json
import wave
import numpy as np
root=Path(__file__).resolve().parent
metrics={}
for name in ('voice','band'):
    path=root/f'{name}.wav'
    with wave.open(str(path),'rb') as f:
        rate,channels=f.getframerate(),f.getnchannels()
        assert f.getsampwidth()==2
        raw=np.frombuffer(f.readframes(f.getnframes()),dtype='<i2').reshape(-1,channels)
    assert not np.any(np.abs(raw.astype(float))>=32767), 'Clipped source; rerender quieter'
    x=raw.astype(float)/32768
    if name=='voice':
        x=x.mean(axis=1,keepdims=True)
    # One gain for the whole take, never normalise individual phrases.
    x*=min(.12/np.sqrt(np.mean(x*x)),.9/np.max(np.abs(x)))
    pcm=np.rint(x*32767).astype('<i2')
    with wave.open(str(path),'wb') as f:
        f.setnchannels(x.shape[1]);f.setsampwidth(2);f.setframerate(rate);f.writeframes(pcm.tobytes())
    metrics[name]=dict(duration_seconds=len(x)/rate,sample_rate=rate,channels=x.shape[1],peak=float(np.max(np.abs(pcm.astype(float)))/32768),rms=float(np.sqrt(np.mean((pcm.astype(float)/32768)**2))),clipped_samples=int(np.sum(np.abs(pcm.astype(float))>=32767)))
(root/'metrics.json').write_text(json.dumps(metrics,indent=2)+'\n')
print(json.dumps(metrics,indent=2))
