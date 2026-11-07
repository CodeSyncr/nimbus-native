// The clipboard in a browser: pages served over https (or localhost) only.
NN.plugin("clipboard", {
  set: async ({ text }) => {
    if (!navigator.clipboard) NN.fail("unavailable", "the clipboard needs https");
    await navigator.clipboard.writeText(text);
  },
  get: async () => {
    if (!navigator.clipboard?.readText) NN.fail("unavailable", "this browser can't paste from code");
    return { text: await navigator.clipboard.readText() };
  },
});
