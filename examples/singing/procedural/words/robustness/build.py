from pathlib import Path
import yaml
root=Path('examples/singing/procedural/words/robustness');root.mkdir(exist_ok=True)
for name,beat,notes in [('slow',.7,['e3','g3','a3']),('quick',.3,['e3','g3','a3']),('low',.5,['c3','d3','e3']),('high',.5,['a3','b3','c4'])]:
 s=dict(name=name,key='C',length=beat,instrument='piano',volume=16000,humanize=0,reverb=0,plain=True,scat=dict(channel='C1',delivery='playful',breaths=0,breathiness=.05),sections=[dict(C1=[notes[0],notes[1],'2:'+notes[2],'z'],C2=['5:z'],scat=dict(syllables=['hey','you','stay','']))])
 (root/f'{name}.yaml').write_text(yaml.safe_dump(s,sort_keys=False))
