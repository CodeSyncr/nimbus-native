// Browsers don't say whether a screen reader is on; announcing goes through
// a live region, which screen readers read out.
(() => {
  let live = null;
  NN.plugin("screenreader", {
    enabled: () => NN.fail("unavailable", "browsers don't say whether a screen reader is on"),
    announce: ({ text }) => {
      if (!live) {
        live = document.createElement("div");
        live.setAttribute("aria-live", "polite");
        live.style.cssText = "position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0 0 0 0)";
        document.body.appendChild(live);
      }
      live.textContent = "";
      setTimeout(() => { live.textContent = text; }, 50);
    },
  });
})();
