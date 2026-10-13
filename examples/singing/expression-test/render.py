"""Controlled chorus audition: native Leif versus explicitly shaped performance."""
import sys,json,copy,math,wave,argparse
from pathlib import Path
import numpy as np
ROOT=Path(__file__).resolve().parents[3]
sys.path.insert(0,str(ROOT/'scripts/singing'))
import diffsinger_onnx as ds
OUT=Path(__file__).resolve().parent
plan=json.loads((ROOT/'.local/singing/sun-in-my-shoes-plan.json').read_text())
phrases=copy.deepcopy(plan['phrases'][4:8])
origin=phrases[0]['notes'][0]['start']-.3
for ph in phrases:
 for n in ph['notes']:n['start']-=origin
base=dict(version=1,sample_rate=44100,language='en',duration=15.6,phrases=phrases)
expressive=copy.deepcopy(base)
for ph in expressive['phrases']:
 # Hold each line's final word into part of its written rest, retaining a breath.
 ph['notes'][-1]['duration']+=.23
(OUT/'original.json').write_text(json.dumps(base,indent=2))
(OUT/'expressive.json').write_text(json.dumps(expressive,indent=2))
config=json.loads((ROOT/'.local/singing/leif-lush.json').read_text())['command']
args=argparse.Namespace(voicebank=config[config.index('--voicebank')+1],vocoder=config[config.index('--vocoder')+1],speaker='src/leif_lush',steps=20,threads=4,seed=42,controls='{}',prosody='neural',expression=1.,output='')
original_prepare=ds.ProsodyPredictor.prepare
accents={'hey','hand','light','sun','night','sky','blue','swings','you'}
def shaped_prepare(self,notes,table,dictionary):
 start,tokens,durations,f0,symbols,detail=original_prepare(self,notes,table,dictionary)
 times=start+np.arange(f0.shape[1])*self.timestep
 cents=np.zeros(f0.shape[1])
 for n in notes:
  t=times-n['start']; duration=n['duration'];mask=(t>=0)&(t<duration)
  if n['lyric'].lower() in accents:
   # Small upward arrival into the note, not a change to the written melody.
   cents[mask]+=-35*np.maximum(0,1-t[mask]/.12)**2
  if duration>.6:
   onset=.32;fade=np.clip((t-onset)/.16,0,1)
   cents[mask]+=17*fade[mask]*np.sin(2*np.pi*5.2*(t[mask]-onset))
 f0=f0*np.exp2(cents[None]/1200).astype(np.float32)
 detail['performance_cents']=cents.tolist()
 return start,tokens,durations,f0,symbols,detail

def read(path):
 with wave.open(str(path)) as w:return np.frombuffer(w.readframes(w.getnframes()),dtype='<i2').astype(float)/32768

def write(path,a):
 with wave.open(str(path),'wb') as w:w.setparams((1,2,44100,0,'NONE','not compressed'));w.writeframes(np.round(a*32767).astype('<i2').tobytes())
audios=[]
for label,request in [('original',base),('expressive',expressive)]:
 ds.ProsodyPredictor.prepare=shaped_prepare if label=='expressive' else original_prepare
 args.output=str(OUT/(label+'_raw.wav'));ds.render(args,request)
 audio=read(args.output)
 if label=='expressive':
  ts=np.arange(len(audio))/44100; db=np.full(len(audio),-3.)
  for ph in request['phrases']:
   for i,n in enumerate(ph['notes']):
    start=n['start']-.045;end=ph['notes'][i+1]['start']-.045 if i+1<len(ph['notes']) else n['start']+n['duration']+.06
    mask=(ts>=start)&(ts<end);u=np.clip((ts[mask]-start)/(end-start),0,1)
    level=2.5 if n['lyric'].lower() in accents else (-3 if n['lyric'].lower() in {'the','to','a','and','in','that'} else 0)
    # Strong initial stress, sustained line endings, relaxed releases.
    db[mask]=level+1.2*np.sin(np.pi*u)-3*np.clip((u-.8)/.2,0,1)
  width=441;db=np.convolve(np.pad(db,(220,220),mode='edge'),np.ones(width)/width,mode='valid')
  audio*=10**(db/20)
 audios.append(audio)
rms=[np.sqrt(np.mean(a*a)) for a in audios];gains=[min(rms)/r for r in rms]
peak=max(np.max(np.abs(a*g)) for a,g in zip(audios,gains));gains=[g*min(1,.88/peak) for g in gains]
metrics=[]
for label,a,g in zip(['original','expressive'],audios,gains):
 a*=g;write(OUT/(label+'.wav'),a)
 metrics.append(dict(file=label+'.wav',seconds=len(a)/44100,peak=float(np.max(np.abs(a))),clipped_samples=int(np.sum(np.abs(a)>=1))))
(OUT/'metrics.json').write_text(json.dumps(metrics,indent=2))
print(metrics)
