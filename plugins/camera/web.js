// Photos through the browser's file chooser: phones offer the camera or the
// library, computers a file. Each photo is scaled and made a JPEG here.
(() => {
  async function photo(file, { maxSize, quality }) {
    const img = await createImageBitmap(file);
    const max = maxSize || 2000, k = Math.min(1, max / Math.max(img.width, img.height));
    const w = Math.round(img.width * k), h = Math.round(img.height * k);
    const c = document.createElement("canvas");
    c.width = w; c.height = h;
    c.getContext("2d").drawImage(img, 0, 0, w, h);
    const blob = await new Promise((r) => c.toBlob(r, "image/jpeg", quality || 0.85));
    return { data: await NN.base64(blob), width: w, height: h };
  }
  async function one(o, capture) {
    const [f] = await NN.pick({ accept: "image/*", capture });
    if (!f) NN.fail("cancelled", "no photo was chosen");
    return photo(f, o);
  }
  NN.plugin("camera", {
    take: (o) => one(o, "environment"),
    pick: (o) => one(o),
    pickMany: async (o) => {
      let files = await NN.pick({ accept: "image/*", multiple: true });
      if (!files.length) NN.fail("cancelled", "no photo was chosen");
      if (o.limit > 0) files = files.slice(0, o.limit);
      const out = [];
      for (const f of files) out.push(await photo(f, o));
      return out;
    },
    pickVideo: async ({ maxBytes }) => {
      const [f] = await NN.pick({ accept: "video/*" });
      if (!f) NN.fail("cancelled", "no video was chosen");
      if (f.size > (maxBytes || 50 << 20)) NN.fail("failed", "the video is too large");
      const url = URL.createObjectURL(f), v = document.createElement("video");
      v.preload = "metadata";
      v.src = url;
      const seconds = await new Promise((r) => { v.onloadedmetadata = () => r(v.duration || 0); v.onerror = () => r(0); });
      URL.revokeObjectURL(url);
      return { data: await NN.base64(f), type: f.type || "video/mp4", seconds: isFinite(seconds) ? seconds : 0 };
    },
  });
})();
