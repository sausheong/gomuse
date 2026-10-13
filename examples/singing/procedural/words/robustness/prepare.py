from pathlib import Path
import wave,json
import numpy as np
root=Path(__file__).resolve().parent
metrics={}
for p in root.glob('*.wav'):
 with wave.open(str(p),'rb') as f:
  sr,ch=f.getframerate(),f.getnchannels();x=np.frombuffer(f.readframes(f.getnframes()),dtype='<i2').astype(float).reshape(-1,ch).mean(axis=1)/32768
 x*=.1/np.sqrt(np.mean(x*x));assert max(abs(x))<1
 pcm=np.rint(x*32767).astype('<i2')
 with wave.open(str(p),'wb') as f:f.setnchannels(1);f.setsampwidth(2);f.setframerate(sr);f.writeframes(pcm.tobytes())
 metrics[p.stem]=dict(duration_seconds=len(x)/sr,sample_rate=sr,channels=1,peak=float(max(abs(pcm.astype(float)))/32768),clipped_samples=int((abs(pcm.astype(float))>=32767).sum()))
(root/'metrics.json').write_text(json.dumps(metrics,indent=2)+'\n')
