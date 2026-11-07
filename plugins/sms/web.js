// An sms: link, which only phones (and Macs with Messages) can open.
NN.plugin("sms", {
  available: () => ({ available: /Android|iPhone|iPad|Macintosh/.test(navigator.userAgent) }),
  compose: ({ to = [], body }) => {
    const apple = /iPhone|iPad|Macintosh/.test(navigator.userAgent);
    NN.launch("sms:" + to.map(encodeURIComponent).join(",") + (body ? (apple ? "&" : "?") + "body=" + encodeURIComponent(body) : ""));
    return { status: "unknown" };
  },
});
