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

  const BLEND = ["source-over", "multiply", "screen", "overlay", "darken", "lighten", "color-dodge", "color-burn",
    "hard-light", "soft-light", "difference", "exclusion", "hue", "saturation", "color", "luminosity",
    "destination-out", "source-atop", "lighter"];
  const pictures = new Map(); // url → Image, shared by every canvas
  const clocks = new WeakMap(); // canvas → {defs, st: Map(id → state), raf}

  // eased is ui/animate.go's: where a run is t ms in, and how long a run takes.
  function eased(e, bounce, t, ms) {
    if (e === 4) {
      const z = Math.min(Math.max(1 - bounce, 0.05), 1), w = 2 * Math.PI;
      if (z === 1) { const len = ms * 9.24 / w, u = Math.min(t, len) / ms; return [1 - Math.exp(-w * u) * (1 + w * u), len]; }
      const wd = w * Math.sqrt(1 - z * z), len = ms * Math.max(1, Math.log(1000 * (1 + z * w / wd)) / (z * w)), u = Math.min(t, len) / ms;
      return [1 - Math.exp(-z * w * u) * (Math.cos(wd * u) + z * w / wd * Math.sin(wd * u)), len];
    }
    const x = Math.min(Math.max(t / ms, 0), 1);
    if (e === 1) return [x, ms];
    if (e === 2) return [x * x * x, ms];
    if (e === 3) return [1 - (1 - x) ** 3, ms];
    return [x < 0.5 ? 4 * x * x * x : 1 - 4 * (1 - x) ** 3, ms];
  }
  // value is an animation's number now; it marks the state done after its last run.
  function value(a, s, now) {
    const t = now - s.start - a.delay;
    if (a.kind) { // a tween, or a drag let go
      if (s.live != null) return s.live;
      if (t <= 0) return s.from;
      const [v, len] = eased(a.ease, a.bounce, t, a.ms);
      if (t >= len) { s.done = true; return s.to; }
      return s.from + (s.to - s.from) * v;
    }
    if (t < 0) return 0;
    const len = eased(a.ease, a.bounce, 0, a.ms)[1];
    let i = Math.floor(t / len), v;
    if (a.repeat >= 0 && i > a.repeat) { s.done = true; i = a.repeat; v = 1; }
    else v = eased(a.ease, a.bounce, t - i * len, a.ms)[0];
    return a.reverse && i % 2 ? 1 - v : v;
  }
  // shade runs a shader (ui.Shader, as WebGL 2) over box at pw×ph pixels,
  // on one canvas shared by every canvas; null where it can't.
  let gl = null;
  const programs = new Map();
  function shade(src, u, box, pw, ph) {
    if (gl === null) gl = document.createElement("canvas").getContext("webgl2", { premultipliedAlpha: true }) || false;
    if (!gl) return null;
    let prog = programs.get(src);
    if (prog === undefined) {
      const part = (type, text) => { const sh = gl.createShader(type); gl.shaderSource(sh, text); gl.compileShader(sh);
        if (!gl.getShaderParameter(sh, gl.COMPILE_STATUS)) { console.warn("shader:", gl.getShaderInfoLog(sh)); return null; } return sh; };
      const vs = part(gl.VERTEX_SHADER, "#version 300 es\nout vec2 nn_uv;\nvoid main() { vec2 c = vec2((gl_VertexID << 1) & 2, gl_VertexID & 2); " +
        "nn_uv = vec2(c.x, 1.0 - c.y); gl_Position = vec4(c * 2.0 - 1.0, 0.0, 1.0); }"), fs = part(gl.FRAGMENT_SHADER, src);
      prog = null;
      if (vs && fs) {
        prog = gl.createProgram(); gl.attachShader(prog, vs); gl.attachShader(prog, fs); gl.linkProgram(prog);
        if (!gl.getProgramParameter(prog, gl.LINK_STATUS)) { console.warn("shader:", gl.getProgramInfoLog(prog)); prog = null; }
      }
      programs.set(src, prog);
    }
    if (!prog) return null;
    const c = gl.canvas;
    if (c.width !== pw || c.height !== ph) { c.width = pw; c.height = ph; }
    gl.viewport(0, 0, pw, ph);
    gl.useProgram(prog);
    if (u.length) gl.uniform1fv(gl.getUniformLocation(prog, "nn_u"), u);
    gl.uniform4f(gl.getUniformLocation(prog, "nn_box"), box[0], box[1], box[2], box[3]);
    gl.clearColor(0, 0, 0, 0); gl.clear(gl.COLOR_BUFFER_BIT);
    gl.drawArrays(gl.TRIANGLES, 0, 3);
    return c;
  }

  function tick(el) {
    const c = clocks.get(el);
    if (!c) return;
    c.raf = 0;
    paint(el, true);
  }

  // touch takes a canvas's taps and drags (Draw.Touch): the topmost tagged
  // shape under the finger, as last drawn; a drag moves its number here,
  // frame by frame, and Go hears where it was let go.
  function touch(el, c) {
    const drags = c.defs.filter((a) => a.kind === 2);
    el.style.touchAction = drags.some((a) => a.repeat === 0) ? (drags.some((a) => a.repeat === 1) ? "none" : "pan-y") : drags.length ? "pan-x" : "manipulation";
    if (el.dataset.touch) return;
    el.dataset.touch = "1";
    const at = (e) => { const r = el.getBoundingClientRect(); return [e.clientX - r.left, e.clientY - r.top]; };
    let down = null;
    el.addEventListener("pointerdown", (e) => {
      const [x, y] = at(e), dpr = devicePixelRatio || 1, g = el.getContext("2d");
      let shape = 0;
      for (let i = c.hits.length - 1; i >= 0 && !shape; i--) {
        const h = c.hits[i];
        g.save(); g.setTransform(h.m); g.lineWidth = h.w; g.lineCap = g.lineJoin = "round";
        if (h.w ? g.isPointInStroke(h.p, x * dpr, y * dpr) : g.isPointInPath(h.p, x * dpr, y * dpr, h.even ? "evenodd" : "nonzero")) shape = h.n;
        g.restore();
      }
      const a = c.defs.find((a) => a.kind === 2 && a.reverse === shape && shape) || c.defs.find((a) => a.kind === 2 && a.reverse === 0);
      down = { x, y, shape, moved: false, a };
      if (a) { const st = c.st.get(a.id); down.v0 = value(a, st, performance.now()); el.setPointerCapture(e.pointerId); }
    });
    el.addEventListener("pointermove", (e) => {
      if (!down) return;
      const [x, y] = at(e);
      if (Math.hypot(x - down.x, y - down.y) > 8) down.moved = true;
      if (!down.a || !down.moved) return;
      const a = down.a, st = c.st.get(a.id), delta = (a.repeat === 1 ? y - down.y : x - down.x) / c.s;
      st.live = Math.min(Math.max(down.v0 + delta, a.min), a.max);
      if (!c.raf) c.raf = requestAnimationFrame(() => tick(el));
    });
    const up = (e, cancel) => {
      if (!down) return;
      const d = down; down = null;
      if (d.a && d.moved) {
        const st = c.st.get(d.a.id), v = st.live;
        st.live = null; st.from = st.to = v; st.done = true;
        send({ t: "native", tag: tagOf(el), name: "drag", data: d.a.id + ":" + v });
      } else if (!d.moved && !cancel) {
        send({ t: "native", tag: tagOf(el), name: "tap", data: d.shape + "," + (d.x - c.ox) / c.s + "," + (d.y - c.oy) / c.s });
      }
    };
    el.addEventListener("pointerup", (e) => up(e, false));
    el.addEventListener("pointercancel", (e) => up(e, true));
  }

  function paint(el, again) {
    let d;
    try { d = JSON.parse(el.dataset.draw); } catch (_) { return; }
    const dpr = devicePixelRatio || 1, W = el.clientWidth, H = el.clientHeight;
    if (!W || !H || !d.w || !d.h) return;
    const now = performance.now();
    // The animations: new ones (or a new run) start now; a tween starts toward a new target from where it is.
    let c = clocks.get(el);
    if (!c) { c = { st: new Map(), raf: 0 }; clocks.set(el, c); }
    const defs = [], A = d.a || [];
    for (let i = 0; i + 12 <= A.length; i += 12) {
      const a = { id: A[i], run: A[i + 1], kind: A[i + 2], ms: A[i + 3], delay: A[i + 4], ease: A[i + 5], bounce: A[i + 6],
        repeat: A[i + 7], reverse: A[i + 8], to: A[i + 9], min: A[i + 10], max: A[i + 11] };
      let s = c.st.get(a.id);
      if (!s || (a.kind === 0 && s.run !== a.run)) s = { run: a.run, start: now, from: a.to, to: a.to, done: a.kind !== 0 };
      else if (a.kind && s.to !== a.to && s.live == null) { const at = value(c.defs.find((x) => x.id === a.id) || a, s, now); s.from = at; s.to = a.to; s.start = now; s.done = false; }
      c.st.set(a.id, s);
      defs.push(a);
    }
    c.defs = defs;
    c.d = d; c.hits = [];
    if (d.t) touch(el, c);
    const vals = defs.map((a) => { const s = c.st.get(a.id), was = s.done, v = value(a, s, now); if (s.done && !was && a.kind === 0) send({ t: "native", tag: tagOf(el), name: "anim", data: a.id + ":" + a.run }); return v; });
    if (!again || el.width !== Math.round(W * dpr)) { el.width = Math.round(W * dpr); el.height = Math.round(H * dpr); }
    const top = el.getContext("2d"), s = Math.min(W / d.w, H / d.h), k = dpr * s;
    c.s = s; c.ox = (W - d.w * s) / 2; c.oy = (H - d.h * s) / 2;
    top.setTransform(1, 0, 0, 1, 0, 0);
    top.clearRect(0, 0, el.width, el.height);
    top.save(); // this frame's state ends with it
    top.setTransform(k, 0, 0, k, dpr * (W - d.w * s) / 2, dpr * (H - d.h * s) / 2);
    top.lineCap = top.lineJoin = "round";
    let g = top, n = 0, ci = 0, si = 0, p = new Path2D(), even = false, pending = null, binds = null, trim = null;
    let shadow = null; // [dx, dy, blur, color] in canvas units: Canvas 2D's own ignore the transform
    const stack = [];
    const strs = d.s || [];
    const v = () => { const j = n++, b = binds && binds[j - argBase]; return b ? b[1] + (b[2] - b[1]) * vals[b[0]] : (d.n[j] || 0); };
    let argBase = 0;
    const col = () => d.c[ci++];
    const str = () => strs[si++] || "";
    const setShadow = () => {
      if (!shadow || shadow[3] === "transparent") { g.shadowColor = "transparent"; return; }
      g.shadowOffsetX = shadow[0] * k; g.shadowOffsetY = shadow[1] * k; g.shadowBlur = shadow[2] * k; g.shadowColor = shadow[3];
    };
    const gradient = () => {
      const kind = v(), x0 = v(), y0 = v(), x1 = v(), y1 = v(), m = v(), stops = [];
      for (let j = 0; j < m; j++) stops.push(v());
      const gr = kind === 1 ? g.createRadialGradient(x0, y0, 0, x0, y0, Math.hypot(x1 - x0, y1 - y0))
        : kind === 2 && g.createConicGradient ? g.createConicGradient(Math.atan2(y1 - y0, x1 - x0), x0, y0)
        : g.createLinearGradient(x0, y0, x1, y1);
      for (let j = 0; j < m; j++) gr.addColorStop(Math.min(Math.max(stops[j], 0), 1), col());
      return gr;
    };
    const stroke = (w, style) => {
      g.lineWidth = w; g.strokeStyle = style;
      if (trim) {
        const [len, a, b] = trim, vis = Math.max(Math.min(b, 1) - Math.max(a, 0), 0) * len;
        trim = null;
        if (vis <= 0) return;
        g.save(); g.setLineDash([vis, 2 * len]); g.lineDashOffset = vis + 2 * len - Math.max(a, 0) * len; g.stroke(p); g.restore();
      } else g.stroke(p);
    };
    const fresh = () => { p = new Path2D(); even = false; };
    // hit remembers a tagged shape as drawn, for taps: its path, transform and stroke.
    let tagNext = 0;
    const hit = (w) => { if (tagNext) c.hits.push({ n: tagNext, p, m: g.getTransform(), w, even }); tagNext = 0; };
    for (const cmd of d.k) {
      if (cmd === 28) { // DrawBind: for the next command's numbers
        (pending = pending || [])[d.n[n]] = [d.n[n + 1], d.n[n + 2], d.n[n + 3]];
        n += 4;
        continue;
      }
      binds = pending; pending = null; argBase = n;
      if (cmd === 0) p.moveTo(v(), v());
      else if (cmd === 1) p.lineTo(v(), v());
      else if (cmd === 2) p.quadraticCurveTo(v(), v(), v(), v());
      else if (cmd === 3) p.bezierCurveTo(v(), v(), v(), v(), v(), v());
      else if (cmd === 4) p.closePath();
      else if (cmd === 5) { const x = v(), y = v(), r = v(); p.moveTo(x + r, y); p.arc(x, y, r, 0, 2 * Math.PI); }
      else if (cmd === 6) { const x = v(), y = v(), w = v(), h = v(), r = Math.min(v(), w / 2, h / 2); p.roundRect(x, y, w, h, Math.max(r, 0)); }
      else if (cmd === 7) { g.fillStyle = col(); g.fill(p, even ? "evenodd" : "nonzero"); hit(0); fresh(); }
      else if (cmd === 8) { const w = v(); stroke(w, col()); hit(w); fresh(); }
      else if (cmd === 9) { g.fillStyle = gradient(); g.fill(p, even ? "evenodd" : "nonzero"); hit(0); fresh(); }
      else if (cmd === 10) {
        const x = v(), y = v(), r = v(), a0 = v() * Math.PI / 180, sw = v() * Math.PI / 180;
        p.moveTo(x + r * Math.cos(a0), y + r * Math.sin(a0)); p.arc(x, y, r, a0, a0 + sw, sw < 0);
      }
      else if (cmd === 11) { const x = v(), y = v(), w = v(), h = v(); p.moveTo(x + w, y + h / 2); p.ellipse(x + w / 2, y + h / 2, w / 2, h / 2, 0, 0, 2 * Math.PI); }
      else if (cmd === 12) even = true;
      else if (cmd === 13) { const w = v(); stroke(w, gradient()); hit(w); fresh(); }
      else if (cmd === 29) tagNext = v();
      else if (cmd === 30) {
        const x = v(), y = v(), w = v(), h = v(), m = v(), u = [];
        for (let j = 0; j < m; j++) u.push(v());
        const fallback = col(), src = str();
        // Run it over the box at the canvas's resolution, then draw that inside the path.
        const t = g.getTransform(), k = Math.hypot(t.a, t.b);
        const out = w > 0 && h > 0 && shade(src, u, [x, y, w, h], Math.min(Math.ceil(w * k), 2048), Math.min(Math.ceil(h * k), 2048));
        if (out) { g.save(); g.clip(p, even ? "evenodd" : "nonzero"); g.drawImage(out, 0, 0, out.width, out.height, x, y, w, h); g.restore(); }
        else { g.fillStyle = fallback; g.fill(p, even ? "evenodd" : "nonzero"); }
        hit(0); fresh();
      }
      else if (cmd === 14) { g.clip(p, even ? "evenodd" : "nonzero"); fresh(); }
      else if (cmd === 15) { g.save(); stack.push({ shadow }); }
      else if (cmd === 16) {
        const top2 = stack.pop();
        if (!top2) continue;
        if (top2.layer) {
          const child = g;
          g = top2.parent;
          g.save(); g.setTransform(1, 0, 0, 1, 0, 0); g.globalAlpha = top2.opacity; g.shadowColor = "transparent";
          g.drawImage(child.canvas, 0, 0); g.restore();
        } else g.restore();
        shadow = top2.shadow;
      }
      else if (cmd === 17) {
        const op = Math.min(Math.max(v(), 0), 1), layer = document.createElement("canvas");
        layer.width = el.width; layer.height = el.height;
        const lg = layer.getContext("2d");
        lg.setTransform(g.getTransform()); lg.lineCap = g.lineCap; lg.lineJoin = g.lineJoin; lg.setLineDash(g.getLineDash());
        lg.globalCompositeOperation = "source-over";
        stack.push({ layer: true, parent: g, opacity: op, shadow });
        g = lg; setShadow();
      }
      else if (cmd === 18) g.translate(v(), v());
      else if (cmd === 19) g.rotate(v() * Math.PI / 180);
      else if (cmd === 20) g.scale(v(), v());
      else if (cmd === 21) g.globalCompositeOperation = BLEND[v()] || "source-over";
      else if (cmd === 22) {
        // Canvas 2D's shadows ignore the transform: scale them to the canvas here.
        shadow = [v(), v(), v(), col()];
        setShadow();
      }
      else if (cmd === 23) { const m = v(), ls = []; for (let j = 0; j < m; j++) ls.push(v()); g.setLineDash(ls); }
      else if (cmd === 24) { g.lineCap = ["round", "butt", "square"][v()] || "round"; g.lineJoin = ["round", "miter", "bevel"][v()] || "round"; }
      else if (cmd === 25) trim = [v(), v(), v()];
      else if (cmd === 26) {
        const url = str(), x = v(), y = v(), w = v(), h = v(), fit = v();
        let img = pictures.get(url);
        if (!img) { img = new Image(); img.decoding = "async"; img.src = url; pictures.set(url, img); }
        if (!img.complete || !img.naturalWidth) { img.addEventListener("load", () => paint(el), { once: true }); continue; }
        const iw = img.naturalWidth, ih = img.naturalHeight;
        if (fit === 2) g.drawImage(img, x, y, w, h);
        else {
          const sc = fit === 1 ? Math.min(w / iw, h / ih) : Math.max(w / iw, h / ih), dw = iw * sc, dh = ih * sc;
          g.save(); g.beginPath(); g.rect(x, y, w, h); g.clip();
          g.drawImage(img, x + (w - dw) / 2, y + (h - dh) / 2, dw, dh); g.restore();
        }
      }
      else if (cmd === 27) {
        const text = str(), family = str(), x = v(), y = v(), w = v(), size = v(), weight = v(), align = v(), lines = v();
        let color = col();
        if (color === "transparent") color = getComputedStyle(el).color;
        g.save();
        g.font = weight + " " + size + "px " + (family ? JSON.stringify(family) + ", " : "") + "system-ui, -apple-system, sans-serif";
        g.fillStyle = color; g.textBaseline = "top";
        const out = [];
        for (const para of text.split("\n")) {
          if (!(w > 0)) { out.push(para); continue; }
          let line = "";
          for (const word of para.split(" ")) {
            const t = line ? line + " " + word : word;
            if (line && g.measureText(t).width > w) { out.push(line); line = word; } else line = t;
          }
          out.push(line);
        }
        const shown = lines > 0 ? out.slice(0, lines) : out;
        if (lines > 0 && out.length > lines && w > 0) {
          let last = shown[lines - 1];
          while (last && g.measureText(last + "…").width > w) last = last.slice(0, -1);
          shown[lines - 1] = last.trimEnd() + "…";
        }
        shown.forEach((t, j) => {
          const tw = g.measureText(t).width, dx = w > 0 ? (align === 1 ? (w - tw) / 2 : align === 2 ? w - tw : 0) : 0;
          g.fillText(t, x + dx, y + j * size * 1.2);
        });
        g.restore();
      }
      else break;
    }
    while (stack.length) { const t = stack.pop(); if (t.layer) { const child = g; g = t.parent; g.save(); g.setTransform(1, 0, 0, 1, 0, 0); g.globalAlpha = t.opacity; g.drawImage(child.canvas, 0, 0); g.restore(); } else g.restore(); }
    top.restore();
    if (defs.some((a) => !c.st.get(a.id).done) && !c.raf) c.raf = requestAnimationFrame(() => tick(el));
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
