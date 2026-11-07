// Players are <audio> elements. Browsers play only after the page has had a
// tap.
(() => {
  const players = new Map();
  let next = 0;
  const get = (id) => players.get(id) || NN.fail("not_found", "no player " + id);
  NN.plugin("audioplayer", {
    open: async ({ url, data, type }) => {
      const a = new Audio();
      a.preload = "auto";
      a.src = url || URL.createObjectURL(new Blob([NN.bytes(data)], { type: type || "audio/mpeg" }));
      await new Promise((ok, no) => {
        a.addEventListener("loadedmetadata", ok, { once: true });
        a.addEventListener("error", () => no(Object.assign(new Error("can't load " + (url || "the sound")), { nnCode: "failed" })), { once: true });
      });
      const id = "web-" + ++next;
      players.set(id, a);
      return { id };
    },
    play: ({ id }) => get(id).play(),
    pause: ({ id }) => { get(id).pause(); },
    seek: ({ id, seconds }) => { get(id).currentTime = seconds; },
    volume: ({ id, volume }) => { get(id).volume = Math.min(1, Math.max(0, volume)); },
    rate: ({ id, rate }) => { get(id).playbackRate = rate; },
    loop: ({ id, loop }) => { get(id).loop = loop; },
    status: ({ id }) => {
      const a = get(id);
      return { position: a.currentTime, duration: isFinite(a.duration) ? a.duration : 0, playing: !a.paused && !a.ended, ended: a.ended };
    },
    close: ({ id }) => {
      const a = players.get(id);
      if (!a) return;
      a.pause();
      if (a.src.startsWith("blob:")) URL.revokeObjectURL(a.src);
      players.delete(id);
    },
  });
})();
