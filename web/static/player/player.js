const $ = id => document.getElementById(id);
const defaultScore = `name: Window light
key: C
length: 0.4
instrument: electric-piano
volume: 5000
humanize: 0
reverb: 0.12
instruments: {C1: clarinet, C2: electric-piano, C3: upright-bass}
performance: {C1: {articulation: legato}}
tone: {C2: {tremolo: {rate: 4, depth: 0.3}}}
sections:
  - C1: [e4, g4, 2:a4, g4, e4, 2:c4]
    C2: [4:c3-e3-g3-b3, 4:f3-a3-c4-e4]
    C3: [2:c2, 2:g2, 2:f2, 2:c3]
    drums: [2:brushsweep, brushtap, z, 2:brushsweep, brushtap, z]
  - C1: [d4, f4, 2:g4, e4, d4, 2:c4]
    C2: [4:d3-f3-a3-c4, 4:c3-e3-g3-b3]
    C3: [2:d2, 2:a2, 2:c2, 2:g2]
    drums: [2:brushsweep, brushtap, z, 2:brushsweep, brushtap, z]
`;
let initial = defaultScore;
try { initial = sessionStorage.getItem('muse-player-score') || initial; sessionStorage.removeItem('muse-player-score'); } catch {}
$('score').value = initial;
let workletReady, renderedScore = null;
let ctx, node, gain, pan, filter, worker, chunks = [], frames = 0, duration = 0, finished = false, playing = false, started = false, name = 'Muse', generation = 0;
const time = seconds => `${Math.floor(seconds / 60)}:${String(Math.floor(seconds % 60)).padStart(2, '0')}`;
function status(message) { $('status').textContent = message; }
function stopWorker() { worker?.terminate(); worker = null; }
function play(value) { playing = value; node?.port.postMessage({ type: 'play', value }); $('play').textContent = value ? 'Pause' : 'Play'; }
function clearAudio() {
  ++generation; stopWorker(); node?.disconnect(); node?.port.close(); node = null;
  gain?.disconnect(); pan?.disconnect(); filter?.disconnect(); chunks = []; frames = 0; duration = 0; finished = started = false; playing = false;
  for (const id of ['play','stop','seek','cancel','download']) $(id).disabled = true;
  $('seek').value = $('seek').max = 0; $('buffer').value = 0; $('clock').textContent = '0:00 / 0:00';
  renderedScore = null; updateEditState(); drawWaveform();
  $('buffer-label').textContent = 'No audio rendered.'; $('play').textContent = 'Play';
  $('player-label').textContent = 'Playback';
}
function controls() {
  if (!ctx || !gain) return;
  gain.gain.setTargetAtTime(Number($('volume').value) / 100, ctx.currentTime, .02);
  pan.pan.setTargetAtTime(Number($('pan').value), ctx.currentTime, .02);
  filter.frequency.setTargetAtTime(Number($('tone').value), ctx.currentTime, .02);
}
async function render() {
  clearAudio(); const token = generation, mode = $('mode').value;
  renderedScore = $('score').value; updateEditState();
  $('mode').disabled = true;
  $('error').hidden = true; $('render').disabled = true; $('cancel').disabled = false;
  status('Loading the player engine…');
  try {
    if (!window.isSecureContext || !window.AudioWorkletNode) throw new Error('Browser playback needs HTTPS or localhost and a browser with AudioWorklet support.');
    ctx ||= new AudioContext({ sampleRate: 44100 });
    await ctx.resume();
    workletReady ||= ctx.audioWorklet.addModule('processor.js');
    await workletReady;
    if (token !== generation) return;
    node = new AudioWorkletNode(ctx, 'muse-player', { outputChannelCount: [2] });
    filter = ctx.createBiquadFilter(); filter.type = 'lowpass'; filter.Q.value = .707;
    gain = ctx.createGain(); pan = ctx.createStereoPanner();
    node.connect(filter).connect(gain).connect(pan).connect(ctx.destination); controls();
    node.port.onmessage = ({ data }) => {
      if (token !== generation) return;
      $('seek').value = Math.min(data.seconds, duration); drawWaveform();
      $('clock').textContent = `${time(data.seconds)} / ${time(duration)}`;
      if (data.ended && playing) { play(false); status('Playback complete. Seek or press Play to listen again.'); }
      else if (data.waiting && playing) status('Buffering audio… Playback will continue automatically.');
      else if (playing) status(finished ? 'Playing.' : 'Playing while the score renders…');
    };
    worker = new Worker('worker.js');
    worker.onerror = event => fail(event.message || 'The audio worker stopped.', token);
    worker.onmessage = ({ data: d }) => {
      if (token !== generation) return;
      if (d.type === 'error') return fail(d.message, token);
      if (d.type === 'info') {
        name = d.info.name || 'Muse'; duration = d.info.duration; $('seek').max = duration;
        status(d.info.scat ? 'Preparing the scat vocal track…' : 'Rendering audio…');
        $('player-label').textContent = name;
      }
      if (d.type === 'chunk') {
        chunks.push(d.pcm); frames += d.pcm.length / 2; drawWaveform();
        const copy = d.pcm.slice(); node.port.postMessage({ type: 'chunk', pcm: copy }, [copy.buffer]);
        duration = Math.max(duration, frames / 44100); $('seek').max = duration;
        $('buffer').value = Math.min(1, frames / 44100 / Math.max(duration, .01));
        $('buffer-label').textContent = `${time(frames / 44100)} rendered`;
        if (mode === 'progressive' && !started && frames >= 44100) begin();
      }
      if (d.type === 'done') {
        finished = true; duration = d.duration; $('seek').max = duration; $('buffer').value = 1;
        node.port.postMessage({ type: 'done' }); $('download').disabled = false; $('cancel').disabled = true; $('render').disabled = false; $('mode').disabled = false;
        $('buffer-label').textContent = `All audio ready · rendered in ${(d.elapsed / 1000).toFixed(1)} s`;
        if (!started) begin(); stopWorker();
      }
    };
    worker.postMessage({ type: 'render', score: renderedScore, mode });
  } catch (error) { fail(error.message, token); }
}
function begin() { started = true; for (const id of ['play','stop','seek']) $(id).disabled = false; play(true); }
function fail(message, token) {
  if (token !== generation) return;
  clearAudio(); $('render').disabled = false; $('mode').disabled = false; $('error').textContent = message; $('error').hidden = false; status('Unable to play this score. Correct it and try again.');
}
$('render').onclick = render;
$('play').onclick = async () => {
  await ctx.resume();
  if (finished && Number($('seek').value) >= duration - .05) node.port.postMessage({ type: 'seek', seconds: 0 });
  play(!playing); if (!playing) status('Paused.');
};
$('stop').onclick = () => { play(false); node.port.postMessage({ type: 'seek', seconds: 0 }); status('Stopped.'); };
$('cancel').onclick = () => { clearAudio(); $('render').disabled = false; $('mode').disabled = false; status('Render cancelled. Your score is unchanged.'); $('buffer-label').textContent = 'No audio rendered.'; };
$('seek').oninput = () => { const seconds = Number($('seek').value); node?.port.postMessage({ type: 'seek', seconds }); $('clock').textContent = `${time(seconds)} / ${time(duration)}`; };
for (const id of ['volume','pan','tone']) $(id).oninput = () => {
  $('volume-value').textContent = `${$('volume').value}%`;
  const p = Number($('pan').value); $('pan-value').textContent = p === 0 ? 'Centre' : `${Math.round(Math.abs(p) * 100)}% ${p < 0 ? 'left' : 'right'}`;
  $('tone-value').textContent = Number($('tone').value) === 20000 ? 'Open' : `${(Number($('tone').value) / 1000).toFixed(1)} kHz`;
  controls();
};
$('reset').onclick = () => { $('volume').value = 75; $('pan').value = 0; $('tone').value = 20000; $('volume').oninput(); };
$('mode').onchange = () => { $('mode-hint').textContent = $('mode').value === 'buffered' ? 'Wait for the complete render, including the score’s whole-song mastering.' : 'Playback begins after a short buffer. Volume follows the score; full-song mastering is reserved for the other mode.'; };
$('file').onchange = async () => { const file = $('file').files[0]; if (!file) return; if (file.size > 2 * 1024 * 1024) { $('error').textContent = 'Choose a score smaller than 2 MB.'; $('error').hidden = false; return; } $('score').value = await file.text(); $('error').hidden = true; updateEditState(); status('Score loaded. Press Render & play.'); };
function download(blob, filename) { const url = URL.createObjectURL(blob), a = document.createElement('a'); a.href = url; a.download = filename; a.click(); setTimeout(() => URL.revokeObjectURL(url), 1000); }
$('save-score').onclick = () => download(new Blob([$('score').value], { type: 'text/yaml' }), 'score.yaml');
$('download').onclick = () => {
  const buffer = new ArrayBuffer(44 + frames * 4), view = new DataView(buffer);
  const text = (at, value) => { for (let i = 0; i < value.length; i++) view.setUint8(at + i, value.charCodeAt(i)); };
  text(0,'RIFF'); view.setUint32(4,36 + frames * 4,true); text(8,'WAVE'); text(12,'fmt '); view.setUint32(16,16,true); view.setUint16(20,1,true); view.setUint16(22,2,true); view.setUint32(24,44100,true); view.setUint32(28,176400,true); view.setUint16(32,4,true); view.setUint16(34,16,true); text(36,'data'); view.setUint32(40,frames * 4,true);
  let offset = 44; for (const chunk of chunks) for (const sample of chunk) { view.setInt16(offset, Math.max(-32768, Math.min(32767, Math.round(sample * 32768))), true); offset += 2; }
  download(new Blob([buffer], { type: 'audio/wav' }), `${name.replace(/[^\w -]/g,'').trim() || 'Muse'}.wav`);
};
window.addEventListener('pagehide', () => { clearAudio(); ctx?.close(); });

function updateEditState() {
  const dirty = renderedScore !== null && $('score').value !== renderedScore;
  $('edit-state').dataset.dirty = String(dirty);
  $('edit-state').textContent = dirty ? 'Score changed · render again to hear edits' : renderedScore === null ? 'Ready to render' : 'Audio follows this score';
}
let waveQueued = false;
function drawWaveform() {
  if (waveQueued) return;
  waveQueued = true;
  requestAnimationFrame(() => {
    waveQueued = false;
    const canvas = $('waveform'), width = Math.round(canvas.clientWidth), height = 30;
    const scale = window.devicePixelRatio || 1;
    canvas.width = width * scale; canvas.height = height * scale;
    const c = canvas.getContext('2d'); c.scale(scale, scale);
    const total = Math.max(frames, duration * 44100), position = Number($('seek').value) * 44100;
    for (let x = 0; x < width; x += 4) {
      const start = Math.floor(x / width * total), end = Math.floor((x + 4) / width * total);
      let peak = 0;
      for (let at = start; at < Math.min(end, frames); at += Math.max(1, Math.floor((end-start)/12))) {
        const block = chunks[Math.floor(at/4096)], i = (at%4096)*2;
        if (block) peak = Math.max(peak, Math.abs(block[i] || 0), Math.abs(block[i+1] || 0));
      }
      const bar = Math.max(2, Math.min(28, Math.sqrt(peak)*38));
      c.fillStyle = start < position ? 'oklch(77% .14 73)' : start < frames ? 'oklch(65% .015 75)' : 'oklch(37% .012 65)';
      c.fillRect(x, (height-bar)/2, 2, bar);
    }
  });
}
$('score').addEventListener('input', updateEditState);
window.addEventListener('resize', drawWaveform);
document.addEventListener('keydown', event => {
  if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
    event.preventDefault(); if (!$('render').disabled) render(); return;
  }
  if (event.code === 'Space' && !['INPUT','TEXTAREA','SELECT','BUTTON','SUMMARY'].includes(event.target.tagName) && !event.target.isContentEditable && !$('play').disabled) {
    event.preventDefault(); $('play').click();
  }
});
drawWaveform();
