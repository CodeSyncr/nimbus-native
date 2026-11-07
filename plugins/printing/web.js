// The browser's print dialog: a page prints from a hidden frame, and a PDF
// opens in a tab of its own to print from there.
NN.plugin("printing", {
  print: async ({ name, html, pdf }) => {
    if (pdf) {
      const url = URL.createObjectURL(new Blob([NN.bytes(pdf)], { type: "application/pdf" }));
      if (!window.open(url, "_blank")) NN.fail("failed", "the browser blocked the new tab");
      return;
    }
    if (!html) NN.fail("failed", "nothing to print");
    const frame = document.createElement("iframe");
    frame.style.cssText = "position:fixed;width:0;height:0;border:0;visibility:hidden";
    frame.srcdoc = html;
    document.body.appendChild(frame);
    await new Promise((r) => frame.addEventListener("load", r, { once: true }));
    if (name) frame.contentDocument.title = name;
    frame.contentWindow.print();
    setTimeout(() => frame.remove(), 1000);
  },
});
