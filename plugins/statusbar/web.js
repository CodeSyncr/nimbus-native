// A browser has no status bar of its own: the color becomes the page's
// theme color, which phones' browsers paint around it. Style and hiding
// change nothing.
NN.plugin("statusbar", {
  set: ({ color }) => {
    if (color === undefined) return;
    let m = document.querySelector('meta[name="theme-color"]');
    if (!m) { m = document.createElement("meta"); m.name = "theme-color"; document.head.appendChild(m); }
    const c = color >>> 0;
    m.content = "rgba(" + (c >>> 24) + "," + ((c >>> 16) & 255) + "," + ((c >>> 8) & 255) + "," + ((c & 255) / 255) + ")";
  },
});
