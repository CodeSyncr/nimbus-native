// A mailto: link, which opens the visitor's mail app. It can't attach files
// or tell whether the mail went.
NN.plugin("mailcomposer", {
  available: () => ({ available: true }),
  compose: ({ to = [], cc = [], bcc = [], subject, body, html, files }) => {
    if (files?.length) NN.fail("unavailable", "a browser can't attach files to an email");
    if (html && body) body = new DOMParser().parseFromString(body, "text/html").body.textContent;
    const q = new URLSearchParams();
    if (cc.length) q.set("cc", cc.join(","));
    if (bcc.length) q.set("bcc", bcc.join(","));
    if (subject) q.set("subject", subject);
    if (body) q.set("body", body);
    NN.launch("mailto:" + to.map(encodeURIComponent).join(",") + (q.size ? "?" + q.toString().replaceAll("+", "%20") : ""));
    return { status: "unknown" };
  },
});
