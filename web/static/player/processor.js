// Retain rendered blocks for instant backwards seeks; forward seeks wait for
// that audio to arrive. An underrun outputs silence without advancing the clock.
class MusePlayerProcessor extends AudioWorkletProcessor {
  constructor() {
    super(); this.chunks = []; this.frames = 0; this.position = 0;
    this.playing = false; this.done = false; this.ticks = 0; this.rate = 44100;
    this.port.onmessage = ({ data: d }) => {
      if (d.type === 'chunk') { this.chunks.push(d.pcm); this.frames += d.pcm.length / 2; }
      if (d.type === 'play') this.playing = d.value;
      if (d.type === 'seek') this.position = Math.max(0, d.seconds * this.rate);
      if (d.type === 'done') this.done = true;
    };
  }
  sample(frame, side) {
    const block = this.chunks[Math.floor(frame / 4096)];
    return block?.[(frame % 4096) * 2 + side] || 0;
  }
  process(_inputs, outputs) {
    const [left, right] = outputs[0]; let waiting = false;
    for (let i = 0; i < left.length; i++) {
      if (!this.playing) break;
      const at = Math.floor(this.position);
      if (at >= this.frames || (!this.done && at + 1 >= this.frames)) { waiting = !this.done; break; }
      const fraction = this.position - at;
      for (let side = 0; side < 2; side++) {
        const a = this.sample(at, side), b = this.sample(Math.min(at + 1, this.frames - 1), side);
        (side ? right : left)[i] = a + (b - a) * fraction;
      }
      this.position += this.rate / sampleRate;
    }
    if (++this.ticks % 16 === 0) this.port.postMessage({ type: 'position', seconds: this.position / this.rate, buffered: this.frames / this.rate, waiting, ended: this.done && this.position >= this.frames });
    return true;
  }
}
registerProcessor('muse-player', MusePlayerProcessor);
