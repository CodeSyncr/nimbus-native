// MediaRecorder over the microphone. The type is what the browser records:
// audio/mp4 where it can, else audio/webm.
(() => {
  let rec = null, stream = null, chunks = [], meter = null, ctx = null;
  let ran = 0, since = 0; // seconds recorded before the last start or resume, and when that was
  const seconds = () => ran + (rec?.state === "recording" ? (performance.now() - since) / 1000 : 0);
  const mic = () => navigator.mediaDevices?.getUserMedia({ audio: true }) || NN.fail("unavailable", "this browser has no microphone");
  function end() {
    stream?.getTracks().forEach((t) => t.stop());
    ctx?.close();
    rec = stream = meter = ctx = null;
    chunks = [];
    ran = 0;
  }
  NN.plugin("audiorecorder", {
    request: async () => {
      try { (await mic()).getTracks().forEach((t) => t.stop()); return { granted: true }; }
      catch (e) { if (e.name === "NotAllowedError") return { granted: false }; throw e; }
    },
    start: async () => {
      if (rec) NN.fail("failed", "already recording");
      stream = await mic();
      const type = ["audio/mp4", "audio/webm"].find((t) => MediaRecorder.isTypeSupported(t));
      rec = new MediaRecorder(stream, type ? { mimeType: type } : undefined);
      rec.ondataavailable = (e) => { if (e.data.size) chunks.push(e.data); };
      ctx = new AudioContext();
      meter = ctx.createAnalyser();
      meter.fftSize = 512;
      ctx.createMediaStreamSource(stream).connect(meter);
      rec.start(250);
      since = performance.now();
    },
    pause: () => { if (rec?.state === "recording") { ran = seconds(); rec.pause(); } },
    resume: () => { if (rec?.state === "paused") { rec.resume(); since = performance.now(); } },
    stop: async () => {
      if (!rec) NN.fail("failed", "not recording");
      const s = seconds();
      await new Promise((r) => { rec.onstop = r; rec.stop(); });
      const blob = new Blob(chunks, { type: rec.mimeType });
      end();
      return { data: await NN.base64(blob), type: blob.type.split(";")[0], seconds: s };
    },
    cancel: () => { if (rec && rec.state !== "inactive") rec.stop(); end(); },
    status: () => {
      let level = 0;
      if (meter && rec?.state === "recording") {
        const b = new Float32Array(meter.fftSize);
        meter.getFloatTimeDomainData(b);
        level = Math.min(1, Math.sqrt(b.reduce((s, v) => s + v * v, 0) / b.length) * 4);
      }
      return { recording: rec?.state === "recording", seconds: seconds(), level };
    },
  });
})();
