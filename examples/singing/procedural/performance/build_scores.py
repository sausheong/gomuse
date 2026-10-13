"""Author the same eight-note phrase as two contrasting performances."""
from pathlib import Path
import yaml
root=Path(__file__).resolve().parent
pitches=['g3','a3','c4','a3','g3','e3','g3','c3']
syllables=['doo','bah','wah','doo','bah','doo','wah','doo']
for style in ['tentative','exuberant']:
    soft=style=='tentative'
    durations=[.6,.35,.9,.55,.65,.8,.4,.95] if soft else [.38,.22,.95,.35,.5,.5,.65,1.5]
    lead=.3 if soft else .05
    notes=[f'{lead*2:g}:z']; words=['']; gestures=[{}]
    for i,(pitch,syllable,duration) in enumerate(zip(pitches,syllables,durations)):
        notes.append(f'{duration*2:g}:{pitch}'); words.append(syllable)
        if soft:
            bends=[[-95,-35,0,-18],[-40,15,0,-30],[-65,-10,12,-25,-70],[-55,-10,0,-45],[-25,0,-20,-65],[-80,-20,5,-50],[-45,20,0,-30],[-30,0,-45,-160]][i]
            mouth=[.02,.17,.1,.015] if syllable=='doo' else [.08,.5,.32,.06]
            effort=[.08,.3 if i==2 else .2,.16,.02]
            air=[.62,.28,.4,.85]
            attack,release=.11,.19
        else:
            bends=[[-140,65,0,25],[40,0,35,0],[-160,80,0,35,-15],[65,0,-15,0],[0,60,0,-45],[-70,25,0,-20],[-120,90,30,0],[0,35,0,-90,-330]][i]
            mouth=[.05,.55,.32,.1] if syllable=='doo' else [.1,1,.8,.42]
            effort=[.8,1,.85,.55]
            air=[.08,.02,.05,.18]
            attack,release=.006,.045
        gestures.append(dict(pitch=bends,mouth=mouth,effort=effort,air=air,attack=attack,release=release))
        if soft and i==2:
            notes.append('0.36:z');words.append('');gestures.append({})
    tail=6-lead-sum(durations)-(.18 if soft else 0)
    notes.append(f'{tail*2:g}:z');words.append('');gestures.append({})
    score=dict(name=f'Hand-shaped phrase: {style}',key='C',length=.5,instrument='piano',volume=18000,humanize=0,reverb=0,plain=True,scat=dict(channel='C1',delivery='relaxed',articulation='gentle' if soft else 'crisp',breaths=.3 if soft else .1),sections=[dict(C1=notes,C2=['12:z'],scat=dict(syllables=words,gestures=gestures))])
    (root/f'{style}.yaml').write_text(yaml.safe_dump(score,sort_keys=False))
