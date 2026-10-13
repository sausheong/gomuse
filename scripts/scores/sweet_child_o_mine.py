"""Rebuild the Muse transcription of the seven supplied pages (354–360).
No image upload or external transcription service is used.
"""
from pathlib import Path
from fractions import Fraction

ROOT = Path(__file__).resolve().parents[2]
bars = {}
def notes(s): return s.split()
def eighth(s): return ' '.join('0.5:'+n for n in s.split())
def add(n, page, melody, rh, bass, **kw):
    bars[n] = dict(page=page, C1=notes(melody), C2=notes(rh), C3=notes(bass), **kw)
def intro(n, melody, bass='4:z', rh='4:z'):
    add(n, 354 if n <= 16 else 355, melody, rh, bass, intro=True)
# Printed pitch, D major; each section is one 4/4 measure.
riff = eighth('d4 d5 a4 g4 g5 a4 f5 a4')
criff = eighth('e4 d5 a4 g4 g5 a4 f5 a4')
griff = eighth('g4 d5 a4 g4 g5 a4 f5 a4')
for n,s in enumerate([riff,riff,criff,criff,griff,griff,riff,eighth('d4 d5 a4 g4 g5 f5 e5 a4')],1):
    intro(n,s)
bars[7]['C3']=notes('2:z '+eighth('d3-a3 d3-a3 d3-a3 d3-a3'))
bars[8]['C3']=notes(eighth('d3-a3 d3-a3 d3-a3 d3-a3 d3-a3 d3-a3 d3-a3 a2'))
for n,ch,bs in [
 (9,'d4-f4-a4-d5','1.5:d3 0.5:f3 0.5:f3 0.5:d3 0.5:f3 0.5:g3'),
 (10,'d4-f4-a4-d5','a3 0.5:b3 0.5:a3 0.5:g3 0.5:f3 0.5:e3 0.5:a2'),
 (11,'c4n-e4-g4-c5n','1.5:c3n 0.5:g3 1.5:g3 0.5:c3n'),
 (12,'c4n-e4-g4-c5n',eighth('f3n e3 c3n f3n g3 e3 d3 f2n')),
 (13,'b3-d4-g4-b4','3:g2 0.5:z 0.5:d3'),
 (14,'b3-d4-g4-b4','2:g3 1.5:g3 0.5:d3'),
 (15,'a3-d4-f4-a4','0.5:g3 0.5:f3 0.5:d3 0.5:f3 0.5:f3 0.5:d3 e3'),
 (16,'a3-d4-f4-a4',eighth('d3 d3 d3 d3 d3 d3 e3 f3'))]:
    intro(n,'4:z',bs,'4:'+ch)
for n,mel,bs in [
 (17,riff,'1.5:d2 0.5:g2 1.5:g2 0.5:a2'),
 (18,eighth('d4 d5 a4 g4 g5 a4 f5 a4'),'1.5:a2 0.5:g2 0.5:a2 0.5:c3 0.5:a2 0.5:d2'),
 (19,criff,'1.5:f2n 0.5:e2 1.5:e2 0.5:d2'),
 (20,criff,'1.5:c2n 0.5:c2n 0.5:f2n 0.5:e2 0.5:d2 0.5:c2n'),
 (21,griff,'1.5:g1 0.5:a1 1.5:a1 0.5:b1'),
 (22,griff,eighth('b1 g2 a2 b2 c3n b1 c2n d2')),
 (23,riff,'1.5:d2 0.5:d2 1.5:d2 0.5:d2'),
 (24,'4:a3-d4-f4','0.5:f2 0.5:a2 0.5:b1 0.5:d2 f2 e2')]: intro(n,mel,bs)
# Verse and chorus, pp.355–357. The piano RH is a playable reduction:
# melody doubling is omitted where C1 already carries it; held inner voices
# are represented as chords. Bass rhythms remain independent.
chords={'D':'a3-d4-f4','C':'g3-c4n-e4','G':'g3-b3-d4','A':'a3-c4-e4',
        'D5':'a3-d4','A5':'a3-e4','C5':'g3-c4n','B5':'f3-b3',
        'E5':'b3-e4','F#5':'c4-f4','G5':'d4-g4','Bb5':'f3n-b3b', 'D#5':'a3#-d4#'}
roots={'D':'d2','C':'c2n','G':'g2','A':'a2','D5':'d2','A5':'a2','C5':'c2n','B5':'b1','E5':'e2','F#5':'f2','G5':'g2','Bb5':'b1b','D#5':'d2#'}
def band(n,page,mel,ch,bass=None,rh=None,**kw):
    r=roots[ch]
    if bass is None: bass=f'1.5:{r} 0.5:{r} 2:{r}'
    if rh is None: rh='4:'+chords[ch]
    add(n,page,mel,rh,bass,chord=ch,**kw)
band(25,355,'0.5:a4 0.5:a4 0.5:a4 1.5:a4 0.5:g4 0.5:f4', 'D')
band(26,355,'g4 0.5:a4 0.5:f4 e4 0.5:d4 0.5:g4','D',bass='1.5:d2 0.5:e2 f2 d2')
band(27,355,'0.5:g4 0.5:f4 0.5:d4 1.5:g4 f4','C')
band(28,355,'0.5:g4 0.5:f4 0.5:e4 1.5:d4 0.5:e4 0.5:a4','C')
band(29,355,'0.5:g4 0.5:f4 0.5:d4 1:a4 0.5:a4 0.5:g4 0.5:f4','G')
band(30,355,'0.5:a4 0.5:g4 0.5:f4 0.5:a4 0.5:a4 a4 0.5:a4','G',bass='1.5:g2 0.5:a2 b2 c3n')
band(31,356,'0.5:g4 3.5:f4','D',bass='1.5:d2 0.5:d2 1.5:d2 0.5:e2')
band(32,356,'4:z','D',rh='0.5:a4 1:a3-d4-g4 0.5:a3-d4-a4 2:a3-d4-g4',bass='0.5:d2 0.5:d3 0.5:a2 0.5:d3 e3 e3')
band(33,356,'0.5:a4 a4 1.5:a4 0.5:g4 0.5:e4','D')
band(34,356,'0.5:g4 a4 1.5:f4 a4','D',bass='1.5:d2 0.5:d2 e2 c2n')
band(35,356,'0.5:g4 0.5:f4 0.5:d4 g4 0.5:g4 0.5:f4 0.5:d4','C')
band(36,356,'0.5:g4 a4 g4 0.5:f4 0.5:g4 0.5:e4','C')
band(37,356,'0.5:g4 0.5:f4 0.5:g4 0.5:f4 1.5:g4 0.5:d4','G')
band(38,356,'0.5:f4 0.5:f4 f4 0.5:e4 f4 0.5:e4','G',bass='1.5:g2 0.5:f2 f2 f2')
band(39,356,'0.5:e4 3.5:d4','D',bass='1.5:d2 0.5:d2 1.5:d2 0.5:e2')
band(40,356,'4:z','D',rh='0.5:a3-d4-g4 0.5:f4 0.5:g4 0.5:a3-d4-f4 2:a3-d4-f4',bass='1.5:d2 1:d2 0.5:e2 0.5:f2 0.5:g2')
band(41,356,'3:a4 0.5:g4 0.5:f4','A',bass='0.5:a2-e3-a3 0.5:z 0.5:z 0.5:a2-e3-a3 1.5:a2-e3-a3 0.5:a2')
band(42,356,'0.5:f4 0.5:e4 d4 e4 0.5:e4 0.5:f4','C',bass='0.5:c3n-g3 0.5:z 0.5:z 0.5:c3n-g3 2:c3n-g3')
band(43,357,'0.5:g4 2.5:g4 0.5:z 0.5:z','D')
band(44,357,'4:z','D',rh='0.5:a3-d4-g4 0.5:f4 0.5:e4 0.5:f4 2:d4',bass='1.5:d2 0.5:b1 0.5:e2 0.5:d2 0.5:c2n 0.5:a1')
band(45,357,'1.5:a4 1:g4 f4 0.5:g4','A',bass='0.5:a2-e3-a3 0.5:z 0.5:z 0.5:a2-e3-a3 2:a2-e3-a3')
band(46,357,'0.5:g4 0.5:f4 e4 g4 0.5:a4 0.5:e4','C',bass='0.5:c3n-g3 0.5:z 0.5:z 0.5:c3n-g3 2:c3n-g3')
band(47,357,'4:d4','D5',bass='1.5:d2 0.5:d2 1.5:d2-a2-d3 0.25:c3n 0.25:c3#')
band(48,357,'4:z','D5',rh='3:d4-a4-d5 0.5:z 0.5:a4',bass='0.5:d3 0.5:d3 0.5:d3 1:d3 0.5:d3 0.5:d2 0.5:d2',tocoda=True)
# Instrumental turnaround to D.S. (p.357 last two systems).
for n,ch,mel in [(49,'D','1.5:a4 0.5:g4 1.5:g4 0.5:c5'),(50,'D','1.5:b4 0.5:a4 1.5:a4 0.5:d5'),(51,'C','1.5:c5n 0.5:b4 1.5:b4 0.5:e5'),(52,'C','1.5:d5 0.5:c5n 1.5:c5n 0.5:f5')]:
    band(n,357,mel,ch)
band(53,357,'2:g4 '+eighth('f4 g4 f4 e4'),'G')
band(54,357,'g4 0.5:c5n 0.5:d5 e5 0.5:d5 0.5:f4','G',bass='1.5:g2 0.5:a2 2:b2')
band(55,357,'3.5:d4-f4 0.5:b3','D',bass='1.5:d2 0.5:e2 1.5:e2 0.5:e2')
band(56,357,'1.5:a3-d4-g4 0.5:a3-d4-g4 2:a3-d4-f4','D',bass='0.5:e2 e2 0.5:e2 a2 g2',ds=True)
# Coda, p.358.
band(57,358,'4:z','D',bass=eighth('d3 e2 f2 e2 g2 f2 g2# f2'))
band(58,358,'1.5:a4 0.5:c5n 0.5:c5n d5 0.5:e5','A5',bass='0.5:a2-e3-a3 0.5:z 0.5:z 0.5:a2-e3-a3 1.5:a2-e3-a3 0.5:b2-f3-b3')
band(59,358,'0.5:e5 0.5:e5 e5 0.5:f5 0.5:e5 0.5:d5 0.5:e5','C5',bass='0.5:c3n-g3-c4n 0.5:z 0.5:z 0.5:c3n-g3-c4n 2:c3n-g3-c4n')
band(60,358,'0.5:b4 0.5:c5n 0.5:a4 0.5:g4 2:g4','D5')
band(61,358,'0.5:b4 0.5:a4 0.5:g4 0.5:b4 0.5:c5n 1.5:d5','D5',bass=eighth('d3 d3 d3 d3 d3 d3 d3 d3'))
band(62,358,'1.5:e5 0.5:e5 0.5:e5 e5 0.25:d5 0.25:d5','A5',bass='0.5:a2-e3-a3 0.5:z 0.5:z 0.5:a2-e3-a3 2:a2-e3-a3')
band(63,358,'0.5:a4 0.5:g4 e4 f4 0.5:g4 0.5:a4','C5',bass='0.5:c3n-g3-c4n 0.5:z 0.5:z 0.5:c3n-g3-c4n 2:c3n-g3-c4n')
band(64,358,'0.5:e4 3.5:d4','D5',bass=eighth('d2 d2 d2 d2 d2 d2 d2 d2'))
band(65,358,'4:z','D5',rh=eighth('d3-g3 d3-g3 d3-g3 d3-g3 d3-g3 c3n-f3')+' 0.5:d3#-g3# 0.5:z',bass=eighth('a1 a1 a1 a1 a1 g1')+' 0.5:a1# 0.5:z')
# Ad-lib guitar solo: only its printed accompaniment is transcribed.
# The score supplies no solo notes. C1 is deliberately silent here.
for n,ch in [(66,'E5'),(67,'C5'),(68,'B5'),(69,'A5')]:
    r=roots[ch]; band(n,358,'4:z',ch,rh='4:z',bass=f'1.5:{r}-{chords[ch]} 0.5:{r}-{chords[ch]} 2:{r}-{chords[ch]}',adlib=True)
bars[69]['C3']=notes('1.5:a1-e2-a2 0.5:a1-e2-a2 0.5:a1-e2 0.5:d2 0.5:f2 0.5:e2')
band(70,359,'4:z','A5',rh='4:z',bass=eighth('e2 f2-a2-e3 a2 f2-a2-e3 a2 f2-a2-e3 a2 g2'),adlib=True)
band(71,359,'4:z','A5',rh='4:z',bass=eighth('a2 f2-a2-e3 a2 g2-b2-d3 g2 f2 e2 d2'),adlib=True)
# Four-bar instrumental vamp, repeat four times.
band(72,359,'4:z','E5',rh='4:z',bass='1.5:e2-b2-e3 0.5:e2-b2-e3 1.5:e2-b2-e3 0.5:f2-c3-f3')
band(73,359,'4:z','G5',rh='4:z',bass='1.5:g2-d3-g3 0.5:g2-d3-g3 1.5:g2-d3-g3 0.5:e2-g3')
band(74,359,'4:z','A5',rh='4:z',bass='1.5:a2-e3-a3 0.5:a2-e3-a3 2:a2-e3-a3')
band(75,359,'4:z','C5',rh='4:z',bass='0.5:c3n-e3-g3 0.5:c3n-e3-g3 0.5:c3n-e3-g3 0.5:c3n-e3-g3 d3-f3-a3 g2-b2-d3')
# Vocal refrain played instrumentally. Key signature changes to one sharp:
# key D is retained in YAML, so every C natural is explicit.
band(76,359,'0.5:e4 0.5:d4 0.5:b3 2.5:a3','E5',bass='e2-b2-e3 0.5:z 0.5:e2-b2-e3 z 0.5:z 0.5:f2-c3-f3')
band(77,359,'0.5:e4 0.5:d4 0.5:b3 a3 0.5:a3 g3','G5',bass='g2-d3-g3 0.5:z 0.5:g2-d3-g3 2:z')
band(78,359,'0.5:e4 0.5:d4 0.5:b3 2.5:a3','A5',bass='a2-e3-a3 0.5:z 0.5:a2-e3-a3 2:z')
band(79,359,'4:z','C',rh='0.5:z 0.5:c4n-e4-g4 0.5:c4n-e4-g4 0.5:c4n-e4-g4 d4-f4-a4 b3-d4-g4',bass='0.5:z 0.5:c3n 0.5:c3n 0.5:d3 d3 g2')
band(80,359,'0.5:b4 0.5:a4 0.5:c5n 0.5:a4 2:a4','E5',bass='1.5:e2-b2-e3 0.5:e2-b2-e3 1.5:e2-b2-e3 0.5:f2-c3-f3')
band(81,359,'z 2:e5 e5','G5',bass='1.5:g2-d3-g3 0.5:g2-d3-g3 2:g2-d3-g3')
band(82,359,'0.5:f5 0.5:e5 0.5:c5n 0.5:b4 0.5:b4 1.5:a4','A5',bass='1.5:a2-e3-a3 0.5:a2-e3-a3 2:a2-e3-a3')
band(83,359,'4:z','C',rh='0.5:c4n-e4-g4 0.5:c4n-e4-g4 0.5:c4n-e4-g4 0.5:c4n-e4-g4 d4-f4-a4 b3-d4-g4',bass='0.5:c3n-g3-c4n 0.5:c3n-g3-c4n 0.5:c3n-g3-c4n 0.5:c3n-g3-c4n d3-a3-d4 g2-d3-g3')
band(84,360,'0.5:b4 0.5:a4 0.5:g4 0.5:b4 0.5:a4# 1.5:b4','E5',bass='1.5:e2-b2-e3 0.5:e2-b2-e3 1.5:e2-b2-e3 0.5:f2-c3-f3')
band(85,360,'z 2:b4 0.5:d5 0.5:b4','G5',bass='1.5:g2-d3-g3 0.5:g2-d3-g3 2:g2-d3-g3')
band(86,360,'0.5:b4 0.5:b4 0.5:e5 0.5:b4 0.5:a4 1.5:g4','A5',bass='1.5:a2-e3-a3 0.5:a2-e3-a3 2:a2-e3-a3')
# Six quarter-note triplets; exact decimal remainder keeps the bar at 4 beats.
add(87,360,'0.667:e5 0.667:d5 0.666:b4 0.667:b4b 0.667:a4 0.666:g4','4:z','0.667:e3-b3-e4 0.667:d3-a3-d4 0.666:b2-f3-b3 0.667:b2b-f3n-b3b 0.667:a2-e3-a3 0.666:g2-d3-g3',chord='E5 D5 B5 Bb5 A5 G5')
band(88,360,'2:e4 0.5:z b4n 0.25:a4 0.25:g4','E5',bass='1.5:e2-b2-e3 0.5:e2-b2-e3 1.5:e2-b2-e3 0.5:f2-c3-f3')
band(89,360,'2:e4 0.5:z 0.5:c5n 0.5:b4 0.5:b4','G5',bass='1.5:g2-d3-g3 0.5:g2-d3-g3 2:g2-d3-g3')
band(90,360,eighth('b4 d5 b4 d5 b4 b4 a4 g4'),'A5',bass='4:a2-e3-a3')
band(91,360,eighth('b4 d5 b4 d5 e5 d5 b4 b4'),'C5',rh='2:g3-c4n-e4 2:a3-d4-f4',bass='2:c3n 2:d3',ritardando=0.2)
band(92,360,'0.25:a4 0.25:g4 3.5:e4','E5',bass='4:e2-b2-e3',rh='4:b3-e4-g4')
band(93,360,'1.5:e4 0.5:d4 2:d4','E5',bass='2:e2-g2 2:d2',rh='4:z')
band(94,360,'4:e4','E5',bass='4:e2-b2-e3-g3',rh='4:z',fermata=2)
band(95,360,'4:z','E5',rh='0.5:e3 0.5:b3 0.5:e4 2.5:g4#',bass='4:e2-b2-e3-g3',fermata=2)

# In the coda/outro the two upper staves double the same line, without
# independent inner voices. Retain that printed doubling in the piano.
for n in list(range(58,65))+list(range(76,79))+list(range(80,83))+list(range(84,91))+[92,93,94]:
    bars[n]['C2']=list(bars[n]['C1'])
for n,ch in [(66,'e2-b2-e3'),(67,'c2n-g2-c3n'),(68,'b1-f2-b2')]:
    bars[n]['C3']=notes(f'1.5:{ch} 0.5:{ch} 2:{ch}')
# D.S. goes to the segno at 25; on the second pass jump after 48 to 57.
# Solo repeat: E5 C5 B5 A5 three times, then E5 C5 B5 + two-bar ending 4.
order=list(range(1,57))+list(range(25,49))+list(range(57,66))
for _ in range(3): order+=list(range(66,70))
order+=list(range(66,69))+[70,71]
for _ in range(4): order+=list(range(72,76))
for _ in range(2): order+=list(range(76,80))
for _ in range(4): order+=list(range(80,84))
order+=list(range(84,96))

def beats(seq):
    return sum((Fraction(s.split(':',1)[0]) if ':' in s else Fraction(1)) for s in seq)
for n,b in bars.items():
    for ch in ('C1','C2','C3'):
        assert beats(b[ch])==4,(n,ch,beats(b[ch]))
assert set(order)==set(bars),(set(bars)-set(order))

header='''# Sweet Child O' Mine — draft transcription of supplied pages 354–360.
# Written pitch in D major, then E minor; no recording-style detuning applied.
# Medium Rock is printed without BPM; quarter = 120 is editorial.
# C1: electric guitar opening and instrumental rendering of the vocal melody.
# C2: reduced piano RH harmony; C3: piano LH. No drums are printed or added.
# Opening 1–6 is unaccompanied electric guitar; printed bass enters in bar 7.
# Piano inner voices are reduced; cross-bar ties may rearticulate. Lyrics,
# slurs, grace notes and guitar bends are not encoded. Photo reading is a draft,
# not a verified note-for-note engraving. Source bar numbers are counted here.
# Ad-lib solo (66–71): only printed accompaniment, with no invented lead notes.
# Form: 1–56, D.S. 25–48, coda 57–65, solo 66–69 x3,
# 66–68 then ending 4 (70–71), 72–75 x4, 76–79 x2, 80–83 x4, 84–95.
# The unnumbered repeat at 76–79 is played twice.
# Muse electric-guitar instrument is supplied in internal/muse/electric_guitar.go.
name: Sweet Child O' Mine — electric guitar and piano
key: D
length: 0.5
instrument: piano
instruments: {C1: electric-guitar, C2: piano, C3: piano}
volume: 14000
levels: {C1: 1, C2: 0.45, C3: 0.65}
pan: {C1: -0.12, C2: 0.3, C3: -0.15}
humanize: 0.06
swing: 0
reverb: 0.14
sections:
'''
lines=[header]
for played,n in enumerate(order,1):
    b=bars[n]
    label=f"  # Playback {played}; page {b['page']}, source bar {n}"
    if b.get('chord'): label+='; '+b['chord']
    if b.get('adlib'): label+='; guitar solo ad lib. (lead not written)'
    lines.append(label+'\n')
    dyn='mp' if n<=8 else 'mf'
    lines.append(f'  - dynamic: {dyn}\n')
    for k in ('ritardando','fermata'):
        if k in b: lines.append(f'    {k}: {b[k]}\n')
    for ch in ('C1','C2','C3'): lines.append(f"    {ch}: [{', '.join(b[ch])}]\n")
path=ROOT/'scores/sweet_child_o_mine.yaml'
path.write_text(''.join(lines))
print(f'{path}: {len(bars)} source bars, {len(order)} played bars, {len(order)*4} beats')
