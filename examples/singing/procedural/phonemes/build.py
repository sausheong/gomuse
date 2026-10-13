"""An original 18-second phrase, with manually specified pronunciation."""
from pathlib import Path
import yaml
root=Path(__file__).resolve().parent
rows=[
 [('hey','HH EY','g3',.55),('come','K AH M','e3',.45),('a','AH','g3',.25),('live','L AY V','a3',.65),('feel','F IY L','g3',.65),('a','AH','e3',.25),('lit','L IH T','f3',.35),('tle','AH L','e3',.35),('love','L AH V','c3',.85)],
 [('bright','B R AY T','g3',.65),('morn','M AO R','a3',.5),('ing','N IH NG','g3',.35),('light','L AY T','c4',.65),('car','K AE','a3',.45),('ry','R IY','g3',.35),('me','M IY','e3',.6),('high','HH AY','g3',.55),('er','ER','e3',.55)],
 [('we','W IY','e3',.4),('can','K AE N','g3',.4),('rise','R AY Z','a3',.7),('we','W IY','g3',.4),('can','K AE N','a3',.4),('ride','R AY D','c4',.7),('you','Y UW','b3',.45),('and','AE N D','g3',.35),('I','AY','c3',.8)],
]
sections=[]
for j,row in enumerate(rows):
 notes=['0.4:z'];labels=[''];phones=[[]];gestures=[{}];used=.2
 for i,(label,ph,pitch,dur) in enumerate(row):
  notes.append(f'{dur*2:g}:{pitch}');labels.append(label);phones.append(ph.split());used+=dur
  effort=.35+.18*j+.12*(i%3==0)
  gestures.append(dict(pitch=[-20 if i%3 else -45,12,0,-35 if i==len(row)-1 else 0],effort=[effort*.8,effort,effort*.65]))
  if i==3:
   notes.append('0.5:z');labels.append('');phones.append([]);gestures.append({});used+=.25
 tail=6-used;assert tail>0
 notes.append(f'{2*tail:.6f}:z');labels.append('');phones.append([]);gestures.append({})
 sections.append(dict(C1=notes,C2=['12:z'],scat=dict(syllables=labels,phonemes=phones,gestures=gestures)))
s=dict(name='Come Alive — procedural phoneme study',key='C',length=.5,instrument='piano',volume=18000,humanize=0,reverb=0,plain=True,scat=dict(channel='C1',delivery='playful',clarity=.7,breaths=.1,breathiness=.04),sections=sections)
(root/'come_alive.yaml').write_text(yaml.safe_dump(s,sort_keys=False,allow_unicode=True))
(root/'lyrics.txt').write_text('Hey, come alive, feel a little love.\nBright morning light, carry me higher.\nWe can rise, we can ride, you and I.\n')
