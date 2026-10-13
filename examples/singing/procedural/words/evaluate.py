"""Independent ASR diagnostic: no lyric prompt, hints or vocabulary restriction.
Optional evaluator only. It is not used by Muse or its Go tests.
"""
from pathlib import Path
import json
import hashlib
import argparse
import subprocess
import numpy as np
from faster_whisper import WhisperModel
p=argparse.ArgumentParser()
p.add_argument('--model',default='base.en')
p.add_argument('--beam',type=int,default=5)
p.add_argument('--no-repeat-ngram',type=int,default=0)
p.add_argument('--output',default='examples/singing/procedural/words/asr.json')
p.add_argument('files',nargs='+')
a=p.parse_args()
model=WhisperModel(a.model,device='cpu',compute_type='int8',download_root='.local/word-eval/models')
results=[]
for path in a.files:
    pcm=subprocess.run(["ffmpeg","-v","error","-i",path,"-f","f32le","-ac","1","-ar","16000","-"],check=True,capture_output=True).stdout
    audio=np.frombuffer(pcm,dtype="<f4")
    segments,info=model.transcribe(audio,language='en',beam_size=a.beam,no_repeat_ngram_size=a.no_repeat_ngram,temperature=0,condition_on_previous_text=False,vad_filter=False)
    chunks=[dict(start=s.start,end=s.end,text=s.text,avg_logprob=s.avg_logprob,no_speech_prob=s.no_speech_prob) for s in segments]
    result=dict(file=path,sha256=hashlib.sha256(Path(path).read_bytes()).hexdigest(),model=a.model,beam=a.beam,no_repeat_ngram=a.no_repeat_ngram,prompt=None,segments=chunks)
    results.append(result)
    print(json.dumps(result),flush=True)
Path(a.output).write_text(json.dumps(results,indent=2)+'\n')
