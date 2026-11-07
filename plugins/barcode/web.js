// The camera in a sheet over the page, read by the browser's BarcodeDetector
// (Chrome on Android and macOS; not Safari or Firefox).
NN.plugin("barcode", {
  scan: async () => {
    if (!window.BarcodeDetector) NN.fail("unavailable", "this browser can't read barcodes");
    if (!navigator.mediaDevices?.getUserMedia) NN.fail("unavailable", "this browser has no camera");
    const stream = await navigator.mediaDevices.getUserMedia({ video: { facingMode: "environment" } });
    const sheet = document.createElement("div");
    sheet.style.cssText = "position:fixed;inset:0;z-index:40;background:#000;display:flex;flex-direction:column;align-items:center;justify-content:center";
    const video = document.createElement("video");
    video.muted = true; video.playsInline = true; video.srcObject = stream;
    video.style.cssText = "max-width:100%;max-height:100%";
    const close = document.createElement("button");
    close.textContent = "Cancel";
    close.style.cssText = "position:absolute;top:16px;right:16px;padding:10px 16px;border:0;border-radius:999px;background:rgba(255,255,255,.9);font:inherit;cursor:pointer";
    sheet.append(video, close);
    document.body.appendChild(sheet);
    const end = () => { stream.getTracks().forEach((t) => t.stop()); sheet.remove(); };
    try {
      await video.play();
      const detector = new BarcodeDetector();
      const code = await new Promise((ok, no) => {
        close.onclick = () => no(Object.assign(new Error("nothing was scanned"), { nnCode: "cancelled" }));
        const look = async () => {
          if (!sheet.isConnected) return;
          try { const [c] = await detector.detect(video); if (c) return ok(c); } catch (e) { return no(e); }
          setTimeout(look, 200);
        };
        look();
      });
      return { value: code.rawValue, format: code.format === "qr_code" ? "qr" : code.format.replaceAll("_", "") };
    } finally { end(); }
  },
});
