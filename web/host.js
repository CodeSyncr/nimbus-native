// host.js: the browser side of a Nimbus Native page. It adopts the HTML the
// server rendered, applies the DOM updates the page's session streams
// (web/dom.go documents them), and sends taps, typing and navigation back.
(() => {
  "use strict";
  const sid = document.currentScript.dataset.session;
  const root = document.getElementById("nn");
  const els = new Map(); // tag → element
  const parts = new Map(); // tag → {l, b, x}: the inline style's parts
  for (const el of root.querySelectorAll("[data-t]")) els.set(+el.dataset.t, el);

  // ---- canvases (ui.Canvas): Go's shapes in a w×h space fitted to the box ----

  function paint(el) {
    let d;
    try { d = JSON.parse(el.dataset.draw); } catch (_) { return; }
    const dpr = devicePixelRatio || 1, W = el.clientWidth, H = el.clientHeight;
    if (!W || !H || !d.w || !d.h) return;
    el.width = Math.round(W * dpr); el.height = Math.round(H * dpr);
    const g = el.getContext("2d"), s = Math.min(W / d.w, H / d.h);
    g.setTransform(dpr * s, 0, 0, dpr * s, dpr * (W - d.w * s) / 2, dpr * (H - d.h * s) / 2);
    let n = 0, c = 0, p = new Path2D();
    const v = () => d.n[n++] || 0;
    for (const k of d.k) {
      if (k === 0) p.moveTo(v(), v());
      else if (k === 1) p.lineTo(v(), v());
      else if (k === 2) p.quadraticCurveTo(v(), v(), v(), v());
      else if (k === 3) p.bezierCurveTo(v(), v(), v(), v(), v(), v());
      else if (k === 4) p.closePath();
      else if (k === 5) { const x = v(), y = v(), r = v(); p.moveTo(x + r, y); p.arc(x, y, r, 0, 2 * Math.PI); }
      else if (k === 6) { const x = v(), y = v(), w = v(), h = v(), r = Math.min(v(), w / 2, h / 2); p.roundRect(x, y, w, h, Math.max(r, 0)); }
      else if (k === 7) { g.fillStyle = d.c[c++]; g.fill(p); p = new Path2D(); }
      else if (k === 8) { g.lineWidth = v(); g.lineCap = g.lineJoin = "round"; g.strokeStyle = d.c[c++]; g.stroke(p); p = new Path2D(); }
      else if (k === 9) {
        const radial = v(), x0 = v(), y0 = v(), x1 = v(), y1 = v(), n2 = v(), stops = [];
        for (let j = 0; j < n2; j++) stops.push(v());
        const gr = radial ? g.createRadialGradient(x0, y0, 0, x0, y0, Math.hypot(x1 - x0, y1 - y0)) : g.createLinearGradient(x0, y0, x1, y1);
        for (let j = 0; j < n2; j++) gr.addColorStop(Math.min(Math.max(stops[j], 0), 1), d.c[c++]);
        g.fillStyle = gr; g.fill(p); p = new Path2D();
      }
      else break;
    }
  }
  for (const el of root.querySelectorAll("canvas[data-draw]")) paint(el);
  addEventListener("resize", () => { for (const el of root.querySelectorAll("canvas[data-draw]")) paint(el); });

  // ---- sending ----

  let gone = false;
  function send(m) {
    if (gone) return;
    fetch("/_nn/send?s=" + sid, { method: "POST", body: JSON.stringify(m), keepalive: true,
      headers: { "Content-Type": "application/json" } })
      .then((r) => { if (r.status === 410) expired(); })
      .catch(() => {});
  }
  function expired() {
    if (gone) return;
    gone = true;
    location.reload(); // the session ended: start a fresh one
  }
  const dark = matchMedia("(prefers-color-scheme: dark)");
  function viewport() {
    send({ t: "viewport", w: innerWidth, h: innerHeight, scale: devicePixelRatio || 1, dark: dark.matches });
  }

  // ---- applying updates ----

  const tagOf = (el) => +el.dataset.t;

  function style(tag, el) {
    const p = parts.get(tag);
    if (p) el.style.cssText = p.l + p.b + p.x;
  }

  function create(tag, name, cls) {
    const el = document.createElement(name);
    el.className = cls;
    el.dataset.t = tag;
    const old = els.get(tag);
    if (old) { // replaced (an input became a textarea): keep its place
      for (const a of old.attributes) if (a.name !== "class" && a.name !== "type") el.setAttribute(a.name, a.value);
      old.replaceWith(el);
    }
    els.set(tag, el);
    style(tag, el);
  }

  function children(el, tags) {
    const want = tags.map((t) => els.get(t)).filter(Boolean);
    const have = [...el.children].filter((c) => c.dataset.t);
    if (want.length === have.length && want.every((c, i) => c === have[i])) return;
    for (const c of have) if (!want.includes(c)) c.remove();
    let ref = null; // keep host chrome (bars) where it is: insert after the last placed child
    for (const c of want) {
      const next = ref ? ref.nextElementSibling : firstViewChild(el);
      if (c !== next) el.insertBefore(c, next);
      ref = c;
    }
  }
  function firstViewChild(el) {
    for (const c of el.children) if (c.dataset.t) return c;
    return el.querySelector(":scope > .tabbar");
  }

  function present(tag) {
    const el = els.get(tag);
    if (!el || el.parentElement?.classList.contains("ov")) return;
    const ov = document.createElement("div");
    ov.className = "ov";
    ov.dataset.for = tag;
    ov.addEventListener("click", (e) => { if (e.target === ov) send({ t: "dismissed", tag }); });
    ov.appendChild(el);
    root.appendChild(ov);
  }

  function apply(list) {
    // Sticky scroll views (a chat) follow their content's end as it grows.
    const sticky = [...root.querySelectorAll("[data-stick]")].map((el) => [el, el.scrollHeight]);
    applyAll(list);
    for (const [el, h] of sticky) if (el.scrollHeight > h) el.scrollTo({ top: el.scrollHeight, behavior: "smooth" });
    for (const el of root.querySelectorAll("[data-af]:not([data-focused])")) { el.dataset.focused = ""; el.focus(); }
  }
  function applyAll(list) {
    for (const i of list) {
      if (i.c) create(i.c[0], i.c[1], i.c[2]);
      else if (i.k) { const el = els.get(i.k[0]); if (el) children(el, i.k[1]); }
      else if (i.s) {
        const [tag, k, css] = i.s;
        const p = parts.get(tag) || { l: "", b: "", x: "" };
        p[k] = css;
        parts.set(tag, p);
        const el = els.get(tag);
        if (el) style(tag, el);
      } else if (i.a) {
        const el = els.get(i.a[0]);
        if (el) i.a[2] == null ? el.removeAttribute(i.a[1]) : el.setAttribute(i.a[1], i.a[2]);
        if (el && i.a[1] === "data-draw") paint(el);
      } else if (i.p) {
        const el = els.get(i.p[0]);
        if (el && el[i.p[1]] !== i.p[2]) el[i.p[1]] = i.p[2];
      } else if (i.t) { const el = els.get(i.t[0]); if (el && (el.textContent !== i.t[1] || el.children.length)) el.textContent = i.t[1]; }
      else if (i.h) { // a rich text: its runs as spans
        const el = els.get(i.h[0]);
        if (el) el.replaceChildren(...i.h[1].map(([t, css]) => { const s = document.createElement("span"); s.style.cssText = css; s.textContent = t; return s; }));
      }
      else if (i.d) {
        for (const tag of i.d) {
          const el = els.get(tag);
          if (el) {
            const ov = el.parentElement?.classList.contains("ov") ? el.parentElement : null;
            el.remove();
            ov?.remove();
          }
          els.delete(tag);
          parts.delete(tag);
        }
      } else if (i.r) {
        const el = els.get(i.r);
        if (el && el.parentElement !== root) root.prepend(el);
        for (const c of [...root.children]) if (c !== el && c.dataset.t) c.remove();
      } else if (i.m) present(i.m);
      else if (i.th) {
        const s = document.documentElement.style;
        for (const k of ["bg", "fg", "muted", "ac", "bar", "barfg", "border"]) s.setProperty("--" + k, i.th[k]);
        document.documentElement.style.colorScheme = i.th.dark ? "dark" : "light";
      }
    }
    chrome();
  }

  // Stacks get a bar with the top screen's title and a back button; tabs
  // get a tab bar.
  function chrome() {
    for (const st of root.querySelectorAll(".st")) {
      const screens = [...st.children].filter((c) => c.dataset.t);
      let bar = st.querySelector(":scope > .bar");
      if (!bar) {
        bar = document.createElement("div");
        bar.className = "bar";
        st.prepend(bar);
      }
      const top = screens[screens.length - 1];
      const button = (label, onclick) => {
        const b = document.createElement("button");
        b.textContent = label;
        b.onclick = onclick;
        return b;
      };
      const title = document.createElement("span");
      title.textContent = top?.dataset.title || "";
      const sheet = st.closest(".ov > .md");
      bar.replaceChildren(
        screens.length > 1 ? button("‹ Back", () => send({ t: "popped", tag: tagOf(st), depth: screens.length - 1 }))
          : document.createElement("span"),
        title,
        sheet ? button("Close", () => send({ t: "dismissed", tag: tagOf(sheet) })) : document.createElement("span"));
    }
    for (const tb of root.querySelectorAll(".tb")) {
      const tabs = [...tb.children].filter((c) => c.dataset.t);
      const sel = +(tb.dataset.sel || 0);
      tabs.forEach((c, i) => c.classList.toggle("sel", i === sel));
      let bar = tb.querySelector(":scope > .tabbar");
      if (!bar) {
        bar = document.createElement("div");
        bar.className = "tabbar";
        tb.appendChild(bar);
      }
      bar.replaceChildren(...tabs.map((c, i) => {
        const b = document.createElement("button");
        b.textContent = c.dataset.title || "";
        b.className = i === sel ? "on" : "";
        b.onclick = () => { tb.dataset.sel = i; chrome(); send({ t: "selected", tag: tagOf(tb), index: i }); };
        return b;
      }));
    }
  }

  // ---- events ----

  root.addEventListener("click", (e) => {
    const p = e.target.closest("a.p[data-t]");
    if (!p) return;
    // A link opened in a new tab or window is the browser's business.
    if (p.hasAttribute("href") && (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0)) return;
    e.preventDefault();
    send({ t: "press", tag: tagOf(p) });
  });
  root.addEventListener("keydown", (e) => {
    const p = e.target.closest("a.p[role=button]");
    if (p && (e.key === "Enter" || e.key === " ")) {
      e.preventDefault();
      send({ t: "press", tag: tagOf(p) });
    }
  });
  root.addEventListener("input", (e) => {
    if (e.target.classList?.contains("in")) send({ t: "changed", tag: tagOf(e.target), text: e.target.value });
  });
  // Return sends on an input that submits (Shift+Return still breaks a line).
  root.addEventListener("keydown", (e) => {
    const t = e.target;
    if (e.key !== "Enter" || e.shiftKey || e.isComposing || !t.classList?.contains("in") || t.dataset.submit == null) return;
    e.preventDefault();
    send({ t: "submitted", tag: tagOf(t) });
  });
  root.addEventListener("input", (e) => {
    if (e.target.classList?.contains("sl")) send({ t: "slid", tag: tagOf(e.target), value: +e.target.value });
  });
  // A long press: press and hold on touch screens, a right click with a mouse.
  root.addEventListener("contextmenu", (e) => {
    const p = e.target.closest("a.p[data-lp]");
    if (!p) return;
    e.preventDefault();
    send({ t: "longpress", tag: tagOf(p) });
  });
  // Drags and pinches on views that asked for them.
  const SLOP = 8, fingers = new Map();
  let drag = null, pinch = null;
  root.addEventListener("pointerdown", (e) => {
    fingers.set(e.pointerId, e);
    const t = e.target.closest?.("[data-pan],[data-pinch]");
    if (!t) return;
    if (fingers.size === 2 && t.closest("[data-pinch]")) {
      const [a, b] = fingers.values();
      pinch = { el: t.closest("[data-pinch]"), d: Math.hypot(a.clientX - b.clientX, a.clientY - b.clientY), s: 1 };
      send({ t: "pinched", tag: tagOf(pinch.el), phase: 0, scale: 1 });
      drag = null;
    } else if (fingers.size === 1 && t.closest("[data-pan]")) {
      const el = t.closest("[data-pan]");
      drag = { el, id: e.pointerId, x: e.clientX, y: e.clientY, at: e.timeStamp, lx: e.clientX, ly: e.clientY, vx: 0, vy: 0, on: false };
    }
  });
  addEventListener("pointermove", (e) => {
    if (fingers.has(e.pointerId)) fingers.set(e.pointerId, e);
    if (pinch) {
      const [a, b] = fingers.values();
      if (!b) return;
      pinch.s = Math.hypot(a.clientX - b.clientX, a.clientY - b.clientY) / pinch.d;
      send({ t: "pinched", tag: tagOf(pinch.el), phase: 1, scale: pinch.s });
      return;
    }
    if (!drag || e.pointerId !== drag.id) return;
    const x = e.clientX - drag.x, y = e.clientY - drag.y, dt = (e.timeStamp - drag.at) / 1000;
    if (dt > 0) { drag.vx = (e.clientX - drag.lx) / dt; drag.vy = (e.clientY - drag.ly) / dt; }
    drag.at = e.timeStamp; drag.lx = e.clientX; drag.ly = e.clientY;
    if (!drag.on) {
      if (Math.hypot(x, y) < SLOP) return;
      const way = drag.el.dataset.pan;
      if ((way === "x" && Math.abs(y) > Math.abs(x)) || (way === "y" && Math.abs(x) > Math.abs(y))) { drag = null; return; }
      drag.on = true;
      drag.el.setPointerCapture?.(e.pointerId);
      send({ t: "panned", tag: tagOf(drag.el), phase: 0, x, y, vx: 0, vy: 0 });
      return;
    }
    send({ t: "panned", tag: tagOf(drag.el), phase: 1, x, y, vx: drag.vx, vy: drag.vy });
  });
  const lift = (phase) => (e) => {
    fingers.delete(e.pointerId);
    if (pinch && fingers.size < 2) { send({ t: "pinched", tag: tagOf(pinch.el), phase, scale: pinch.s }); pinch = null; }
    if (drag && e.pointerId === drag.id) {
      if (drag.on) send({ t: "panned", tag: tagOf(drag.el), phase, x: e.clientX - drag.x, y: e.clientY - drag.y, vx: drag.vx, vy: drag.vy });
      drag = null;
    }
  };
  addEventListener("pointerup", lift(2));
  addEventListener("pointercancel", lift(3));
  root.addEventListener("change", (e) => {
    if (e.target.classList?.contains("sw")) send({ t: "toggled", tag: tagOf(e.target), on: e.target.checked });
  });
  let resize;
  addEventListener("resize", () => { clearTimeout(resize); resize = setTimeout(viewport, 100); });
  dark.addEventListener?.("change", viewport);
  addEventListener("popstate", () => send({ t: "navigate", path: location.pathname + location.search }));

  // ---- the stream ----

  const es = new EventSource("/_nn/events?s=" + sid);
  es.onopen = viewport;
  es.onmessage = (e) => {
    const m = JSON.parse(e.data);
    apply(m.i || []);
    if (m.nav) {
      if (m.nav.push && location.pathname + location.search !== m.nav.path) history.pushState(null, "", m.nav.path);
      if (m.nav.title) document.title = m.nav.title;
      if (m.nav.push) root.querySelector(".sc")?.scrollTo(0, 0);
    }
  };
  es.onerror = () => { if (es.readyState === EventSource.CLOSED) expired(); };
  chrome();
})();
