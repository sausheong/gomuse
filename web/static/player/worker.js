/* The Go engine runs here, never on the UI or audio rendering thread. */
importScripts('wasm_exec.js');
let engine;
async function boot() {
  const go = new Go();
  const response = await fetch('muse.wasm');
  if (!response.ok) throw new Error('Player engine is missing. Build it with make player.');
  const { instance } = await WebAssembly.instantiate(await response.arrayBuffer(), go.importObject);
  go.run(instance).catch(error => postMessage({ type: 'error', message: error.message }));
  if (!self.museOpen) throw new Error('The audio engine did not start.');
}
const check = result => { if (result.error) throw new Error(result.error); return result; };
self.onmessage = async ({ data }) => {
  if (data.type !== 'render') return;
  try {
    engine ||= boot();
    await engine;
    const started = performance.now();
    if (data.mode === 'buffered') {
      const result = check(museRender(data.score));
      postMessage({ type: 'info', info: result.info });
      const floats = new Float32Array(result.pcm.buffer);
      for (let from = 0; from < floats.length; from += 8192) {
        const pcm = floats.slice(from, from + 8192);
        postMessage({ type: 'chunk', pcm }, [pcm.buffer]);
      }
      postMessage({ type: 'done', elapsed: performance.now() - started, duration: result.info.duration });
    } else {
      const { info } = check(museOpen(data.score));
      postMessage({ type: 'info', info });
      let done = false, duration = info.duration;
      while (!done) {
        const block = check(museNext(4096));
        done = block.done; duration = block.duration;
        const pcm = new Float32Array(block.pcm.buffer);
        if (pcm.length) postMessage({ type: 'chunk', pcm }, [pcm.buffer]);
        // Yield for browser scheduling and worker cancellation between blocks.
        await new Promise(resolve => setTimeout(resolve, 0));
      }
      postMessage({ type: 'done', elapsed: performance.now() - started, duration });
    }
  } catch (error) { postMessage({ type: 'error', message: error.message }); }
};
