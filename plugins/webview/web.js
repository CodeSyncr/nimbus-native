// An <iframe> in the Native view's box. Many sites refuse to be framed;
// the browser plugin's new tab always works.
NN.view("webview", (el, emit) => {
  const f = document.createElement("iframe");
  f.style.cssText = "width:100%;height:100%;border:0;display:block";
  f.addEventListener("load", () => emit("load", { url: f.getAttribute("src") || "" }));
  el.appendChild(f);
  let last = null;
  return (p) => {
    const key = p.url || "html:" + p.html;
    if (key === last) return;
    last = key;
    if (p.url) { f.removeAttribute("srcdoc"); f.src = p.url; }
    else { f.removeAttribute("src"); f.srcdoc = p.html || ""; }
  };
});
