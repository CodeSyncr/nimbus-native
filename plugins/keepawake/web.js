// The Screen Wake Lock API. Browsers drop the lock when the tab hides, so it
// is taken again when the tab comes back.
(() => {
  let on = false, lock = null;
  const take = async () => { if (on && !lock) { lock = await navigator.wakeLock.request("screen"); lock.addEventListener("release", () => { lock = null; }); } };
  document.addEventListener("visibilitychange", () => { if (document.visibilityState === "visible") take().catch(() => {}); });
  NN.plugin("keepawake", {
    set: async (a) => {
      if (!navigator.wakeLock) NN.fail("unavailable", "this browser can't keep the screen on");
      on = a.on;
      if (on) await take();
      else if (lock) { await lock.release(); lock = null; }
    },
  });
})();
