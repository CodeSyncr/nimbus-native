// Alerts, sheets, date pickers and toasts drawn in the page, in the app's
// theme colors.
(() => {
  const css = `.nn-dlg{border:0;padding:0;border-radius:14px;background:var(--bg,#fff);color:var(--fg,#111);width:min(92vw,360px);box-shadow:0 20px 60px rgba(0,0,0,.3);font:inherit}
.nn-dlg::backdrop{background:rgba(0,0,0,.4)}
.nn-dlg .h{padding:20px 20px 4px;font-weight:600;font-size:17px}.nn-dlg .m{padding:4px 20px 12px;color:var(--muted,#6b7280);white-space:pre-wrap}
.nn-dlg .f{padding:8px 20px 12px}.nn-dlg input{width:100%;font:inherit;padding:10px;border:1px solid var(--border,#e5e7eb);border-radius:8px;background:transparent;color:inherit}
.nn-dlg .b{display:flex;flex-direction:column;border-top:1px solid var(--border,#e5e7eb)}.nn-dlg .b.row{flex-direction:row-reverse}
.nn-dlg button{flex:1;background:none;border:0;padding:14px;font:inherit;color:var(--ac,#2563eb);cursor:pointer}.nn-dlg .b>button+button{border-top:1px solid var(--border,#e5e7eb)}
.nn-dlg .b.row>button+button{border-top:0;border-right:1px solid var(--border,#e5e7eb)}.nn-dlg button.x{color:#dc2626}.nn-dlg button.c{font-weight:600}
.nn-toast{position:fixed;left:50%;bottom:32px;transform:translateX(-50%);background:rgba(20,20,20,.92);color:#fff;padding:10px 18px;border-radius:999px;z-index:30;max-width:86vw;transition:opacity .3s}`;
  const style = document.createElement("style");
  style.textContent = css;
  document.head.appendChild(style);

  // open shows a dialog and resolves with the index of the button pressed;
  // Escape or a click outside picks the cancel button, or -1.
  function open({ title, message, buttons, sheet }, field) {
    return new Promise((done) => {
      const d = document.createElement("dialog");
      d.className = "nn-dlg";
      const add = (cls, text) => { const e = document.createElement("div"); e.className = cls; if (text) e.textContent = text; d.appendChild(e); return e; };
      if (title) add("h", title);
      if (message) add("m", message);
      if (field) add("f").appendChild(field);
      const row = add("b" + (!sheet && buttons.length === 2 ? " row" : ""));
      const cancel = buttons.findIndex((b) => b.style === "cancel");
      const finish = (i) => { d.close(); d.remove(); done(i); };
      buttons.forEach((b, i) => {
        const e = document.createElement("button");
        e.textContent = b.label;
        e.className = b.style === "destructive" ? "x" : b.style === "cancel" ? "c" : "";
        e.onclick = () => finish(i);
        row.appendChild(e);
      });
      d.addEventListener("cancel", (e) => { e.preventDefault(); finish(cancel); });
      d.addEventListener("click", (e) => { if (e.target === d) finish(cancel); });
      document.body.appendChild(d);
      d.showModal();
      (field || row.querySelector("button:not(.c)") || row.firstChild)?.focus();
    });
  }
  const pad = (n) => String(n).padStart(2, "0");
  NN.plugin("dialog", {
    show: async (o) => {
      const i = await open(o);
      if (i < 0) NN.fail("cancelled", "dismissed");
      return i;
    },
    toast: ({ message }) => {
      const t = document.createElement("div");
      t.className = "nn-toast";
      t.setAttribute("role", "status");
      t.textContent = message;
      document.body.appendChild(t);
      setTimeout(() => { t.style.opacity = "0"; setTimeout(() => t.remove(), 300); }, 2500);
    },
    date: async ({ mode, initial }) => {
      const at = new Date(initial), input = document.createElement("input");
      const day = at.getFullYear() + "-" + pad(at.getMonth() + 1) + "-" + pad(at.getDate()), time = pad(at.getHours()) + ":" + pad(at.getMinutes());
      input.type = mode === "time" ? "time" : mode === "datetime" ? "datetime-local" : "date";
      input.value = mode === "time" ? time : mode === "datetime" ? day + "T" + time : day;
      const i = await open({ buttons: [{ label: "Done" }, { label: "Cancel", style: "cancel" }] }, input);
      if (i !== 0 || !input.value) NN.fail("cancelled", "no date was chosen");
      if (mode === "time") { const [h, m] = input.value.split(":"); at.setHours(+h, +m, 0, 0); return at.getTime(); }
      if (mode === "datetime") return new Date(input.value).getTime();
      const [y, mo, d] = input.value.split("-");
      at.setFullYear(+y, +mo - 1, +d);
      return at.getTime();
    },
  });
})();
