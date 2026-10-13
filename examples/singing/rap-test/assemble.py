"""Fit complete local speech phrases to four bars without assigning note pitches."""
from pathlib import Path
import json,subprocess,wave
import numpy as np
out=Path(__file__).resolve().parent
sr=44100;beat=60/92;bar=4*beat;lead=.2
lines=['Rent ate my check, but I still stand tall.','Five flights up, got my name on the wall.','Train runs late, so I move with the crowd.','Small room, big dreams, New York lives loud.']
length=round((lead+4*bar+.35)*sr);audio=np.zeros(length);events=[]
def write(p,a):
 with wave.open(str(p),'wb') as w:w.setparams((1,2,sr,0,'NONE','not compressed'));w.writeframes(np.round(a*32767).astype('<i2').tobytes())
for i,line in enumerate(lines):
 raw=subprocess.check_output(['ffmpeg','-v','error','-i',str(out/f'line{i+1}.aiff'),'-f','f32le','-ac','1','-ar',str(sr),'pipe:1'])
 a=np.frombuffer(raw,dtype='<f4').astype(float)
 assert len(a)>sr//2 and max(abs(a))>.01,'Speech render missing'
 # Trim padding conservatively, retaining 25 ms around audible speech.
 active=np.flatnonzero(abs(a)>max(abs(a))*.003)
 a=a[max(0,active[0]-1102):min(len(a),active[-1]+1103)]
 write(out/f'line{i+1}_trim.wav',a)
 target=bar-[.18,.13,.18,.25][i];factor=len(a)/sr/target
 assert .5<=factor<=2,'Unexpected stretch requirement'
 raw=subprocess.check_output(['ffmpeg','-v','error','-i',str(out/f'line{i+1}_trim.wav'),'-af',f'atempo={factor:.8f}','-f','f32le','-ac','1','-ar',str(sr),'pipe:1'])
 a=np.frombuffer(raw,dtype='<f4').astype(float)
 start=round((lead+i*bar)*sr)
 assert len(a)<round(bar*sr),'Phrase overlaps next bar'
 fade=min(220,len(a)//2);a[:fade]*=np.linspace(0,1,fade);a[-fade:]*=np.linspace(1,0,fade)
 audio[start:start+len(a)]+=a
 events.append(dict(text=line,start_seconds=start/sr,duration_seconds=len(a)/sr,tempo_factor=factor))
audio*=.85/max(abs(audio));write(out/'four_bars_dry.wav',audio)
# A quiet reference click makes the intended grid audible, without adding a beat.
reference=audio.copy()
for i in range(16):
 start=round((lead+i*beat)*sr);t=np.arange(800)/sr
 reference[start:start+800]+=.06*np.sin(2*np.pi*(1400 if i%4==0 else 900)*t)*np.exp(-t/.003)
reference*=min(1,.9/max(abs(reference)));write(out/'four_bars_with_click.wav',reference)
report=dict(engine='macOS Reed (English US) speech',bpm=92,bars=4,events=events,seconds=len(audio)/sr,sample_rate=sr,channels=1,peak=float(max(abs(audio))),clipped_samples=int(np.sum(abs(audio)>=1)))
(out/'performance.json').write_text(json.dumps(report,indent=2)+'\n')
(out/'lyrics.txt').write_text('\n'.join(lines)+'\n')
print(json.dumps(report,indent=2))
