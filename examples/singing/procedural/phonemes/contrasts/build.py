from pathlib import Path
import yaml
root=Path(__file__).resolve().parent
sets={
 'stops':[('buy','B AY'),('die','D AY'),('guy','G AY'),('pie','P AY'),('tie','T AY'),('key','K IY')],
 'airflow':[('fee','F IY'),('see','S IY'),('he','HH IY'),('vie','V AY'),('zoo','Z UW')],
 'liquids':[('low','L OW'),('row','R OW'),('me','M IY'),('knee','N IY'),('we','W IY'),('you','Y UW')],
 'endings':[('lie','L AY'),('light','L AY T'),('ride','R AY D'),('life','L AY F'),('lime','L AY M'),('line','L AY N')],
}
for name,items in sets.items():
 sections=[]
 for label,ph in items:
  sections.append(dict(C1=['0.4:z','1.4:g3','0.6:z'],C2=['2.4:z'],scat=dict(syllables=['',label,''],phonemes=[[],ph.split(),[]])))
 s=dict(name=name,key='C',length=.5,instrument='piano',volume=18000,humanize=0,reverb=0,plain=True,scat=dict(channel='C1',delivery='playful',clarity=.7,breathiness=.04,breaths=0),sections=sections)
 (root/f'{name}.yaml').write_text(yaml.safe_dump(s,sort_keys=False))
