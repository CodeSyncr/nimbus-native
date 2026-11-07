// The browser's file chooser. It opens only soon after a tap.
NN.plugin("filepicker", {
  pick: async ({ types = [], maxBytes }) => {
    const [f] = await NN.pick({ accept: (types || []).join(",") });
    if (!f) NN.fail("cancelled", "nothing was chosen");
    if (f.size > (maxBytes || 20 << 20)) NN.fail("failed", f.name + " is too large");
    return { name: f.name, type: f.type || "application/octet-stream", data: await NN.base64(f) };
  },
});
