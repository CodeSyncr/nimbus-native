// A new tab, which the browser only opens soon after a tap.
NN.plugin("browser", {
  open: ({ url }) => {
    const w = window.open(url, "_blank");
    if (!w) NN.fail("failed", "the browser blocked the new tab");
    w.opener = null;
  },
});
