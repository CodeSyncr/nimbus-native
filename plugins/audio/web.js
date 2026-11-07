// The browser's microphone, with its noise suppression and echo cancelling,
// sampled down to 16 kHz mono for Go's turn detector; clips play through an
// <audio> element, measured for their level.
(() => {
  const RATE = 16000;
  let mic = null, chunks = [], player = null, meter = null, out = null;
  function pcm16(from, rate) {
    // Average down to 16 kHz, then to little-endian int16.
    const step = rate / RATE, n = Math.floor(from.length / step), o = new Int16Array(n);
    for (let i = 0; i < n; i++) {
      let s = 0, a = Math.floor(i * step), b = Math.min(from.length, Math.floor((i + 1) * step));
      for (let j = a; j < b; j++) s += from[j];
      const v = Math.max(-1, Math.min(1, s / Math.max(1, b - a)));
      o[i] = v < 0 ? v * 0x8000 : v * 0x7fff;
    }
    return new Uint8Array(o.buffer);
  }
  function stopMic() {
    if (!mic) return;
    mic.stream.getTracks().forEach((t) => t.stop());
    mic.node.disconnect();
    mic.ctx.close();
    mic = null;
    chunks = [];
  }
  function stopPlayer() {
    if (!player) return;
    player.pause();
    URL.revokeObjectURL(player.src);
    out?.close();
    player = meter = out = null;
  }
  NN.plugin("audio", {
    requestMicrophone: async () => {
      try { (await navigator.mediaDevices.getUserMedia({ audio: true })).getTracks().forEach((t) => t.stop()); return { granted: true }; }
      catch (e) { if (e.name === "NotAllowedError") return { granted: false }; throw e; }
    },
    micStart: async () => {
      if (mic) return;
      if (!navigator.mediaDevices?.getUserMedia) NN.fail("unavailable", "this browser has no microphone");
      const stream = await navigator.mediaDevices.getUserMedia({ audio: { echoCancellation: true, noiseSuppression: true, autoGainControl: false, channelCount: 1 } });
      const ctx = new AudioContext();
      const node = ctx.createScriptProcessor(4096, 1, 1);
      node.onaudioprocess = (e) => { if (chunks.length < 400) chunks.push(pcm16(e.inputBuffer.getChannelData(0), ctx.sampleRate)); };
      ctx.createMediaStreamSource(stream).connect(node);
      node.connect(ctx.destination); // a processor only runs while connected; it writes silence
      await ctx.resume();
      mic = { stream, ctx, node };
    },
    micRead: async () => {
      const all = new Uint8Array(chunks.reduce((n, c) => n + c.length, 0));
      let at = 0;
      for (const c of chunks) { all.set(c, at); at += c.length; }
      chunks = [];
      return { pcm: await NN.base64(all) };
    },
    micStop: stopMic,
    play: async ({ clip }) => {
      stopPlayer();
      const bytes = NN.bytes(clip);
      const type = String.fromCharCode(...bytes.slice(0, 4)) === "RIFF" ? "audio/wav" : String.fromCharCode(...bytes.slice(4, 8)) === "ftyp" ? "audio/mp4" : "audio/mpeg";
      player = new Audio(URL.createObjectURL(new Blob([bytes], { type })));
      out = new AudioContext();
      meter = out.createAnalyser();
      meter.fftSize = 512;
      out.createMediaElementSource(player).connect(meter);
      meter.connect(out.destination);
      await out.resume();
      await player.play(); // a browser plays sound only after the page has had a tap
    },
    playing: () => {
      if (!player || player.ended || player.paused) return { playing: false, level: 0 };
      const b = new Float32Array(meter.fftSize);
      meter.getFloatTimeDomainData(b);
      return { playing: true, level: Math.min(1, Math.sqrt(b.reduce((s, v) => s + v * v, 0) / b.length) * 4) };
    },
    stop: stopPlayer,
    release: () => { stopMic(); stopPlayer(); },
  });
})();
