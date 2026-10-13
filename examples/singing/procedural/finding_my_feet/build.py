"""Compose four individually shaped phrases; preserve earlier listening studies."""
from pathlib import Path
from copy import deepcopy
import yaml
import numpy as np
root=Path(__file__).resolve().parent
studies=root.parent/'performance'
soft=yaml.safe_load((studies/'tentative.yaml').read_text())
bold=yaml.safe_load((studies/'exuberant.yaml').read_text())
def voiced(score):
    section=score['sections'][0]
    return [g for w,g in zip(section['scat']['syllables'],section['scat']['gestures']) if w]
quiet,loud=voiced(soft),voiced(bold)
# Each six-second phrase has its own pacing and contour. The two middle
# responses develop the opening motif rather than repeating an emotion preset.
phrases=[
    dict(pitches=['g3','a3','c4','a3','g3','e3','g3','c3'],durations=[.6,.35,.9,.55,.65,.8,.4,.95],lead=.3,gap=.18,strengths=[0,.05,.2,.08,.05,0,.12,0]),
    dict(pitches=['e3','g3','a3','c4','b3','a3','g3','e3'],durations=[.45,.3,.65,.9,.4,.5,.7,1.2],lead=.15,gap=.12,strengths=[.18,.25,.35,.55,.3,.25,.4,.2]),
    dict(pitches=['f3','a3','c4','d4','c4','a3','b3','g3'],durations=[.3,.25,.7,.8,.35,.45,.75,1.3],lead=.08,gap=.12,strengths=[.45,.6,.7,.85,.6,.5,.75,.5]),
    dict(pitches=['g3','a3','c4','e4','d4','c4','g3','c3'],durations=[.38,.22,.65,.95,.35,.5,.65,1.5],lead=.05,gap=0,strengths=[.8,.9,1,1,.9,.8,.9,.85]),
]
syllables=['doo','bah','wah','doo','bah','doo','wah','doo']
sections=[]
for index,p in enumerate(phrases):
    notes=[f"{p['lead']*2:g}:z"]; words=['']; gestures=[{}]
    for i,(pitch,duration,strength,word) in enumerate(zip(p['pitches'],p['durations'],p['strengths'],syllables)):
        a,b=quiet[i],loud[i]
        g={}
        for field in ['pitch','mouth','effort','air']:
            # Equal-time sampling preserves each authored curve's overall shape.
            x=np.linspace(0,1,max(len(a[field]),len(b[field])))
            av=np.interp(x,np.linspace(0,1,len(a[field])),a[field])
            bv=np.interp(x,np.linspace(0,1,len(b[field])),b[field])
            g[field]=[round(float(v),4) for v in av*(1-strength)+bv*strength]
        for field in ['attack','release']:
            g[field]=round(a[field]*(1-strength)+b[field]*strength,4)
        # Question at the end of the second phrase, answered in the third;
        # reserve the broad falling release for the final cadence.
        if index==1 and i==7:
            g['pitch']=[-25,0,15,95];g['mouth']=[.04,.25,.4,.22]
        if index==2 and i==7:
            g['pitch']=[-40,20,0,-65];g['effort']=[.5,.7,.6,.2]
        if index==3 and i==3:
            g['pitch']=[-85,55,0,18,-8];g['mouth']=[.12,.6,.45,.3,.18]
        notes.append(f'{duration*2:g}:{pitch}');words.append(word);gestures.append(g)
        if i==2 and p['gap']:
            notes.append(f"{p['gap']*2:g}:z");words.append('');gestures.append({})
    tail=round(6-p['lead']-p['gap']-sum(p['durations']),5)
    assert tail>0,(index,tail)
    notes.append(f'{tail*2:g}:z');words.append('');gestures.append({})
    sections.append(dict(C1=notes,C2=['12:z'],scat=dict(syllables=words,gestures=gestures,articulation='gentle' if index<2 else 'crisp',breaths=.22)))
score=dict(name='Finding My Feet — exposed voice',key='C',length=.5,instrument='piano',volume=18000,humanize=0,reverb=0,plain=False,pan={'C1':0,'C2':.25,'C3':-.15},scat=dict(channel='C1',delivery='relaxed'),sections=sections)
(root/'voice.yaml').write_text(yaml.safe_dump(score,sort_keys=False,allow_unicode=True))
band=deepcopy(score);band['name']='Finding My Feet — jazz trio';band['reverb']=.04
band['instruments']={'C3':'bass'};band['levels']={'C1':1,'C2':.17,'C3':.24,'drums':.08}
chords=[['e3-g3-b3','e3-g3-a3','e3-g3-b3'],['g3-c4-e4','g3-b3-e4','g3-c4-e4'],['f3-a3-c4','f3-a3-c4','f3-b3-d4'],['f3-b3-d4','e3-g3-c4','e3-g3-c4']]
bass=[['c2','g2','b2','c3'],['a2','e2','g2','a2'],['d2','a2','g2','b2'],['g2','d2','c2','c2']]
for i,s in enumerate(band['sections']):
    s['C2']=sum(([f'0.6:z',f'2.4:{chord}','z'] for chord in chords[i]),[])
    s['C3']=[f'3:{p}' for p in bass[i]]
    s['drums']=['kick-ride','ride','rim-ride','ride']*3
(root/'band.yaml').write_text(yaml.safe_dump(band,sort_keys=False,allow_unicode=True))
