from pathlib import Path
import yaml
root=Path(__file__).resolve().parent
base=dict(name='Procedural words',key='C',length=.5,instrument='piano',volume=18000,humanize=0,reverb=0,plain=True,scat=dict(channel='C1',delivery='playful',breaths=0,breathiness=.05))
words=['hey','oh','my','you','stay','me','we','see','say','no','so','hi']
for name,items in [('vocabulary',words),('phrase',['hey','you','stay','hey','you','stay'])]:
    s=dict(base)
    s['name']=name
    if name=='vocabulary':
        s['sections']=[dict(C1=['1.6:g3','0.8:z'],C2=['2.4:z'],scat=dict(syllables=[w,''])) for w in items]
    else:
        s['sections']=[]
        for i in range(2):
            s['sections'].append(dict(C1=['0.2:z','1.2:g3','1.2:a3','2.4:c4','z'],C2=['6:z'],scat=dict(syllables=['','hey','you','stay',''],gestures=[{},dict(pitch=[-60,20,0],effort=[.3,.65,.4]),dict(pitch=[-30,10,0],effort=[.4,.6,.4]),dict(pitch=[-50,15,0,-100],effort=[.5,.8,.6,.2]),{}])))
    (root/f'{name}.yaml').write_text(yaml.safe_dump(s,sort_keys=False))
# Separate combinations for an unprompted recognition check. These are not
# supplied as a decoder prompt and do not change the pronunciation model.
for name,words,notes in [
    ('check_one',['oh','my'],['1.4:e3','2:g3']),
    ('check_two',['you','see','me'],['1:e3','1.2:g3','1.8:e3']),
    ('check_three',['we','say','no'],['1:g3','1.2:a3','1.8:g3']),
]:
    s=dict(base);s['name']=name
    total=sum(float(n.split(':')[0]) for n in notes)+1
    s['sections']=[dict(C1=notes+['z'],C2=[f'{total:g}:z'],scat=dict(syllables=words+['']))]
    (root/f'{name}.yaml').write_text(yaml.safe_dump(s,sort_keys=False))
# A new combination covering the five initially proposed words. The final
# syllable spans two different notes, exercising vowel-only continuation.
s=dict(base);s['name']='First words — expressive phrase'
s['sections']=[dict(C1=['0.4:z','1.2:e3','1.6:g3','0.6:z','1.2:g3','1.2:a3','1.2:c4','1.6:a3','z'],C2=['10:z'],scat=dict(syllables=['','oh','my','','hey','you','stay','+',''],gestures=[{},dict(pitch=[-20,0,-15]),dict(pitch=[0,20,-40]),{},dict(pitch=[-60,20,0],effort=[.3,.65,.4]),dict(pitch=[-30,10,0],effort=[.4,.6,.4]),dict(pitch=[-35,20,0],effort=[.5,.8,.7]),dict(pitch=[0,-15,-110],effort=[.7,.5,.15]),{}]))]
(root/'first_words.yaml').write_text(yaml.safe_dump(s,sort_keys=False))
