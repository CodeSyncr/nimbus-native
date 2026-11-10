// The live camera in a <video>, from getUserMedia; Take draws its frame.
(() => {
  let live = null; // the View on screen: { video, front }
  const stop = (s) => s?.getTracks().forEach((t) => t.stop());

  NN.view("cameraview", (el, emit) => {
    const v = document.createElement("video");
    v.muted = true; v.playsInline = true; v.autoplay = true;
    v.style.cssText = "width:100%;height:100%;object-fit:cover;background:#000;display:block";
    el.appendChild(v);
    const me = { video: v, front: null, torch: false };
    let starting = null;
    const torch = () => {
      const t = v.srcObject?.getVideoTracks()[0];
      if (t?.getCapabilities?.().torch) t.applyConstraints({ advanced: [{ torch: me.torch && !me.front }] }).catch(() => {});
    };
    const start = async (front) => {
      stop(v.srcObject);
      v.srcObject = null;
      if (!navigator.mediaDevices?.getUserMedia) return emit("error", { code: "unavailable" });
      try {
        const s = await navigator.mediaDevices.getUserMedia({ video: { facingMode: front ? "user" : "environment" }, audio: false });
        if (!el.isConnected || me.front !== front) return stop(s);
        v.srcObject = s;
        v.style.transform = front ? "scaleX(-1)" : ""; // a mirror, as on phones
        await v.play().catch(() => {});
        live = me;
        torch();
        emit("ready", {});
      } catch (e) {
        emit("error", { code: e.name === "NotAllowedError" || e.name === "SecurityError" ? "denied" : "unavailable" });
      }
    };
    const gone = new MutationObserver(() => {
      if (el.isConnected) return;
      gone.disconnect();
      stop(v.srcObject);
      if (live === me) live = null;
    });
    gone.observe(document.body, { childList: true, subtree: true });
    return (p) => {
      me.torch = !!p.torch;
      if (me.front !== !!p.front) { me.front = !!p.front; starting = start(me.front); }
      else starting?.then(torch);
    };
  });

  NN.plugin("cameraview", {
    request: async () => {
      if (!navigator.mediaDevices?.getUserMedia) NN.fail("unavailable", "this browser has no camera");
      try { stop(await navigator.mediaDevices.getUserMedia({ video: true })); return { granted: true }; }
      catch (e) { if (e.name === "NotAllowedError") return { granted: false }; throw e; }
    },
    take: async ({ maxSize, quality }) => {
      const v = live?.video;
      if (!v?.videoWidth || !v.isConnected) NN.fail("unavailable", "no camera view is showing");
      let w = v.videoWidth, h = v.videoHeight;
      if (maxSize > 0 && Math.max(w, h) > maxSize) { const k = maxSize / Math.max(w, h); w = Math.round(w * k); h = Math.round(h * k); }
      const c = document.createElement("canvas");
      c.width = w; c.height = h;
      c.getContext("2d").drawImage(v, 0, 0, w, h); // the photo isn't mirrored, as on phones
      const blob = await new Promise((r) => c.toBlob(r, "image/jpeg", quality > 0 ? quality : 0.85));
      return { data: await NN.base64(blob), width: w, height: h };
    },
  });
})();
