// Links open in a new tab; mail, phone and text links go to the system.
NN.plugin("linking", {
  open: ({ url }) => {
    if (/^https?:/i.test(url)) {
      const w = window.open(url, "_blank");
      if (!w) NN.fail("failed", "the browser blocked the new tab");
      w.opener = null;
    } else NN.launch(url);
    return true;
  },
  canOpen: ({ url }) => /^(https?|mailto|tel|sms):/i.test(url),
});
