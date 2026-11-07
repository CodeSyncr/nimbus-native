// The Web Share API; desktop browsers without it fail with unavailable.
NN.plugin("share", {
  share: async ({ title, text, url, files }) => {
    if (!navigator.share) NN.fail("unavailable", "this browser can't share");
    const data = {};
    if (title) data.title = title;
    if (text) data.text = text;
    if (url) data.url = url;
    if (files?.length) {
      data.files = files.map((f) => new File([NN.bytes(f.data)], f.name, { type: f.type }));
      if (!navigator.canShare?.(data)) NN.fail("unavailable", "this browser can't share these files");
    }
    // Like the phones, answer once the sheet is up, not when it closes.
    const shown = navigator.share(data).then(() => null, (e) => e);
    const e = await Promise.race([shown, new Promise((r) => setTimeout(() => r(null), 300))]);
    if (e && e.name !== "AbortError") throw e;
  },
});
