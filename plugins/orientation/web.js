// Screen Orientation: browsers lock it only in full screen, and mostly on
// Android.
NN.plugin("orientation", {
  set: async ({ mode }) => {
    const o = screen.orientation;
    if (!mode) { o?.unlock?.(); return; }
    if (!o?.lock) NN.fail("unavailable", "this browser can't lock the orientation");
    try { await o.lock(mode); }
    catch (e) { NN.fail(e.name === "NotSupportedError" ? "unavailable" : "denied", "browsers lock the orientation only in full screen: " + e.message); }
  },
});
