// scene in a browser: the same pipeline as the phones, in WebGL 2. If
// shadows are on, a depth-only pass from the first light; then the scene,
// lit, textured, and shadowed by sampling that map around each point. Spin
// turns here every frame; with nothing spinning it draws only on a change.
(() => {
  // What Go has sent, by key, for every scene on the page. Each scene's GL
  // context uploads its own copy.
  const store = { meshes: new Map(), textures: new Map() };
  const hosts = new Set();
  const arrived = () => hosts.forEach((h) => h.use());

  NN.plugin("scene", {
    mesh: ({ key, vertices, indices }) => {
      const v = NN.bytes(vertices), i = NN.bytes(indices);
      if (v.length < 32 || i.length < 4) NN.fail("failed", "an empty mesh");
      store.meshes.set(key, { v: new Float32Array(v.buffer, v.byteOffset, v.byteLength / 4), i: new Uint32Array(i.buffer, i.byteOffset, i.byteLength / 4) });
      arrived();
    },
    texture: async ({ key, data }) => {
      const b = NN.bytes(data);
      const type = b[0] === 0x89 ? "image/png" : "image/jpeg";
      store.textures.set(key, await createImageBitmap(new Blob([b], { type })));
      arrived();
    },
  });

  const vs = `#version 300 es
layout(location = 0) in vec3 p; layout(location = 1) in vec3 n; layout(location = 2) in vec2 uv;
uniform mat4 model, normalM, vp, lightVP;
out vec3 world; out vec3 nrm; out vec2 tc; out vec4 lp;
void main() {
  vec4 w = model * vec4(p, 1.0);
  gl_Position = vp * w;
  world = w.xyz; nrm = (normalM * vec4(n, 0.0)).xyz; tc = uv; lp = lightVP * w;
}`;
  const fs = `#version 300 es
precision highp float; precision highp sampler2DShadow;
in vec3 world; in vec3 nrm; in vec2 tc; in vec4 lp;
uniform sampler2D tex; uniform sampler2DShadow map;
uniform vec4 color, params, eye, ambient, dirs[4], colors[4]; uniform int count;
out vec4 frag;
float lit(vec3 n, vec3 l) {
  if (ambient.y < 0.5) return 1.0;
  vec3 q = lp.xyz / lp.w; // Go's light space keeps depth 0 to 1, as Metal does
  vec2 at = q.xy * 0.5 + 0.5;
  if (at.x < 0.0 || at.x > 1.0 || at.y < 0.0 || at.y > 1.0 || q.z > 1.0) return 1.0;
  float bias = max(0.004 * (1.0 - dot(n, l)), 0.001);
  float ref = q.z - bias; // the shadow pass stored this same 0 to 1 depth
  vec2 texel = 1.0 / vec2(textureSize(map, 0));
  float sum = 0.0;
  for (int x = -1; x <= 1; x++) for (int y = -1; y <= 1; y++) sum += texture(map, vec3(at + vec2(x, y) * texel, ref));
  return sum / 9.0;
}
void main() {
  vec4 base = texture(tex, tc) * color;
  if (base.a < 0.02) discard;
  if (params.y > 0.5) { frag = base; return; }
  vec3 n = normalize(nrm);
  if (!gl_FrontFacing) n = -n;
  vec3 v = normalize(eye.xyz - world);
  vec3 c = base.rgb * ambient.x;
  float shine = params.x;
  for (int i = 0; i < 4; i++) {
    if (i >= count) break;
    vec3 l = -dirs[i].xyz;
    float diff = max(dot(n, l), 0.0);
    vec3 h = normalize(l + v);
    float spec = shine > 0.0 ? pow(max(dot(n, h), 0.0), mix(8.0, 128.0, shine)) * shine : 0.0;
    float shade = (i == 0 && diff > 0.0) ? lit(n, l) : 1.0;
    c += (base.rgb * diff + spec) * colors[i].rgb * shade;
  }
  frag = vec4(c, base.a);
}`;
  const svs = `#version 300 es
layout(location = 0) in vec3 p; uniform mat4 model, lightVP;
void main() { vec4 q = lightVP * model * vec4(p, 1.0); gl_Position = vec4(q.xy, q.z * 2.0 - q.w, q.w); }`;
  const sfs = `#version 300 es
precision mediump float; void main() {}`;

  // Column-major 4×4 matrices, as Go and WebGL keep them.
  const I = () => new Float32Array([1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1]);
  function mul(a, b) {
    const o = new Float32Array(16);
    for (let c = 0; c < 4; c++) for (let r = 0; r < 4; r++) {
      let s = 0;
      for (let k = 0; k < 4; k++) s += a[k * 4 + r] * b[c * 4 + k];
      o[c * 4 + r] = s;
    }
    return o;
  }
  function invertTranspose(m) {
    // The normal matrix: the inverse's transpose, from the upper 3×3.
    const [a, b, c, d, e, f, g, h, i] = [m[0], m[1], m[2], m[4], m[5], m[6], m[8], m[9], m[10]];
    const A = e * i - f * h, B = -(d * i - f * g), C = d * h - e * g;
    const det = a * A + b * B + c * C || 1;
    const inv = [A, -(b * i - c * h), b * f - c * e, B, a * i - c * g, -(a * f - c * d), C, -(a * h - b * g), a * e - b * d].map((x) => x / det);
    return new Float32Array([inv[0], inv[3], inv[6], 0, inv[1], inv[4], inv[7], 0, inv[2], inv[5], inv[8], 0, 0, 0, 0, 1]);
  }
  function axisAngle(ax, angle) {
    const len = Math.hypot(ax.X, ax.Y, ax.Z) || 1, x = ax.X / len, y = ax.Y / len, z = ax.Z / len;
    const c = Math.cos(angle), s = Math.sin(angle), t = 1 - c;
    return new Float32Array([t * x * x + c, t * x * y + s * z, t * x * z - s * y, 0, t * x * y - s * z, t * y * y + c, t * y * z + s * x, 0,
      t * x * z + s * y, t * y * z - s * x, t * z * z + c, 0, 0, 0, 0, 1]);
  }
  function lookAt(eye, target) {
    let f = [target[0] - eye[0], target[1] - eye[1], target[2] - eye[2]];
    const fl = Math.hypot(...f);
    f = fl > 0 ? f.map((x) => x / fl) : [0, 0, -1];
    const up = Math.abs(f[1]) > 0.999 ? [0, 0, -1] : [0, 1, 0];
    const cross = (a, b) => [a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0]];
    let s = cross(f, up);
    const sl = Math.hypot(...s);
    s = s.map((x) => x / sl);
    const u = cross(s, f), dot = (a, b) => a[0] * b[0] + a[1] * b[1] + a[2] * b[2];
    return new Float32Array([s[0], u[0], -f[0], 0, s[1], u[1], -f[1], 0, s[2], u[2], -f[2], 0, -dot(s, eye), -dot(u, eye), dot(f, eye), 1]);
  }
  function projection(fov, aspect, near, far) {
    const ys = 1 / Math.tan(fov * Math.PI / 360), xs = ys / aspect;
    return new Float32Array([xs, 0, 0, 0, 0, ys, 0, 0, 0, 0, (far + near) / (near - far), -1, 0, 0, 2 * far * near / (near - far), 0]);
  }

  NN.view("scene", (el, emit) => {
    const canvas = document.createElement("canvas");
    canvas.style.cssText = "width:100%;height:100%;display:block;touch-action:none";
    el.appendChild(canvas);
    const gl = canvas.getContext("webgl2", { antialias: true, premultipliedAlpha: false });
    if (!gl) return () => {};
    const compile = (v, f) => {
      const p = gl.createProgram();
      for (const [type, src] of [[gl.VERTEX_SHADER, v], [gl.FRAGMENT_SHADER, f]]) {
        const s = gl.createShader(type);
        gl.shaderSource(s, src);
        gl.compileShader(s);
        if (!gl.getShaderParameter(s, gl.COMPILE_STATUS)) console.warn("scene shader:", gl.getShaderInfoLog(s));
        gl.attachShader(p, s);
      }
      gl.linkProgram(p);
      const u = {};
      for (let i = 0; i < gl.getProgramParameter(p, gl.ACTIVE_UNIFORMS); i++) {
        const name = gl.getActiveUniform(p, i).name.replace("[0]", "");
        u[name] = gl.getUniformLocation(p, name);
      }
      return { p, u };
    };
    const main = compile(vs, fs), shade = compile(svs, sfs);
    const white = gl.createTexture();
    gl.bindTexture(gl.TEXTURE_2D, white);
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, 1, 1, 0, gl.RGBA, gl.UNSIGNED_BYTE, new Uint8Array([255, 255, 255, 255]));
    const depthTexture = (size) => {
      const t = gl.createTexture();
      gl.bindTexture(gl.TEXTURE_2D, t);
      gl.texStorage2D(gl.TEXTURE_2D, 1, gl.DEPTH_COMPONENT32F, size, size);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_COMPARE_MODE, gl.COMPARE_REF_TO_TEXTURE);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_COMPARE_FUNC, gl.LEQUAL);
      return t;
    };
    const noShadow = depthTexture(1);
    let shadowMap = null, shadowFB = null;
    const SHADOW = 2048;

    // This context's copies of the store.
    const meshes = new Map(), textures = new Map();
    function mesh(key) {
      if (meshes.has(key)) return meshes.get(key);
      const m = store.meshes.get(key);
      if (!m) return null;
      const vao = gl.createVertexArray();
      gl.bindVertexArray(vao);
      const vb = gl.createBuffer();
      gl.bindBuffer(gl.ARRAY_BUFFER, vb);
      gl.bufferData(gl.ARRAY_BUFFER, m.v, gl.STATIC_DRAW);
      for (const [loc, size, off] of [[0, 3, 0], [1, 3, 12], [2, 2, 24]]) {
        gl.enableVertexAttribArray(loc);
        gl.vertexAttribPointer(loc, size, gl.FLOAT, false, 32, off);
      }
      const ib = gl.createBuffer();
      gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, ib);
      gl.bufferData(gl.ELEMENT_ARRAY_BUFFER, m.i, gl.STATIC_DRAW);
      gl.bindVertexArray(null);
      const out = { vao, count: m.i.length };
      meshes.set(key, out);
      return out;
    }
    function texture(key) {
      if (!key) return white;
      if (textures.has(key)) return textures.get(key);
      const img = store.textures.get(key);
      if (!img) return white;
      const t = gl.createTexture();
      gl.bindTexture(gl.TEXTURE_2D, t);
      gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, img);
      gl.generateMipmap(gl.TEXTURE_2D);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.REPEAT);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.REPEAT);
      textures.set(key, t);
      return t;
    }

    // The scene, and where Orbit has taken the camera.
    let props = null, nodes = [], asked = new Set(), started = new Map();
    let goCamera = "", eye = [0, 0, 5], target = [0, 0, 0], yaw = 0, pitch = 0, dist = 5;
    let frame = 0;
    const host = {
      use() {
        if (!props) return;
        const missing = [];
        for (const n of nodes) {
          if (n.mesh && !store.meshes.has(n.mesh) && !asked.has(n.mesh)) { asked.add(n.mesh); missing.push(n.mesh); }
          if (n.texture && !store.textures.has(n.texture) && !asked.has(n.texture)) { asked.add(n.texture); missing.push(n.texture); }
        }
        if (missing.length) emit("need", { keys: missing });
        redraw();
      },
    };
    hosts.add(host);
    const gone = new MutationObserver(() => { if (!el.isConnected) { hosts.delete(host); gone.disconnect(); cancelAnimationFrame(frame); } });
    gone.observe(document.body, { childList: true, subtree: true });

    function redraw() {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(draw);
    }
    function moved() {
      const y = yaw * Math.PI / 180, p = pitch * Math.PI / 180;
      eye = [target[0] + dist * Math.cos(p) * Math.sin(y), target[1] + dist * Math.sin(p), target[2] + dist * Math.cos(p) * Math.cos(y)];
      redraw();
    }

    // A tap reports where; with Orbit, a drag turns the camera and the wheel
    // or a pinch zooms.
    const pointers = new Map();
    let downAt = null, pinch = 0;
    canvas.addEventListener("pointerdown", (e) => { canvas.setPointerCapture(e.pointerId); pointers.set(e.pointerId, [e.clientX, e.clientY]); downAt = [e.clientX, e.clientY, false]; });
    canvas.addEventListener("pointermove", (e) => {
      const last = pointers.get(e.pointerId);
      if (!last) return;
      pointers.set(e.pointerId, [e.clientX, e.clientY]);
      if (downAt && Math.hypot(e.clientX - downAt[0], e.clientY - downAt[1]) > 6) downAt[2] = true;
      if (!props?.orbit) return;
      if (pointers.size === 2) {
        const [a, b] = [...pointers.values()], d = Math.hypot(a[0] - b[0], a[1] - b[1]);
        if (pinch) zoom(d / pinch);
        pinch = d;
        return;
      }
      yaw -= (e.clientX - last[0]) * 0.4;
      pitch = Math.min(85, Math.max(-85, pitch + (e.clientY - last[1]) * 0.4));
      moved();
    });
    const up = (e) => {
      pointers.delete(e.pointerId);
      if (pointers.size < 2) pinch = 0;
      if (downAt && !downAt[2] && pointers.size === 0) {
        const r = canvas.getBoundingClientRect();
        emit("tap", { X: e.clientX - r.left, Y: e.clientY - r.top, W: r.width, H: r.height,
          Eye: { X: eye[0], Y: eye[1], Z: eye[2] }, Target: { X: target[0], Y: target[1], Z: target[2] } });
      }
      if (pointers.size === 0) downAt = null;
    };
    canvas.addEventListener("pointerup", up);
    canvas.addEventListener("pointercancel", (e) => { pointers.delete(e.pointerId); downAt = null; pinch = 0; });
    const zoom = (k) => { dist = Math.min(props.far / 2, Math.max(props.near * 4, dist / k)); moved(); };
    canvas.addEventListener("wheel", (e) => { if (!props?.orbit) return; e.preventDefault(); zoom(Math.exp(-e.deltaY * 0.002)); }, { passive: false });

    function draw(now) {
      if (!props) return;
      const k = devicePixelRatio || 1, r = el.getBoundingClientRect();
      const w = Math.max(1, Math.round(r.width * k)), h = Math.max(1, Math.round(r.height * k));
      if (canvas.width !== w || canvas.height !== h) { canvas.width = w; canvas.height = h; }
      const t = performance.now() / 1000;
      // Where each node is now: parents come first, so a node turning with a
      // spinning one starts from where that one has turned to.
      const placed = [];
      for (const n of nodes) {
        let spin = I();
        const t0 = started.get(n.id);
        if (n.speed && t0 !== undefined && Math.hypot(n.axis.X, n.axis.Y, n.axis.Z) > 0) spin = axisAngle(n.axis, (t - t0) * n.speed * Math.PI / 180);
        const parent = n.parent > 0 && n.parent <= placed.length ? placed[n.parent - 1] : I();
        placed.push(mul(mul(mul(parent, new Float32Array(n.base)), spin), new Float32Array(n.local)));
      }
      const shadows = !!props.shadow, lightVP = props.shadow ? new Float32Array(props.shadow.vp) : I();
      gl.enable(gl.DEPTH_TEST);
      gl.depthFunc(gl.LEQUAL);
      let map = noShadow;
      if (shadows) {
        if (!shadowMap) {
          shadowMap = depthTexture(SHADOW);
          shadowFB = gl.createFramebuffer();
          gl.bindFramebuffer(gl.FRAMEBUFFER, shadowFB);
          gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.TEXTURE_2D, shadowMap, 0);
        }
        gl.bindFramebuffer(gl.FRAMEBUFFER, shadowFB);
        gl.viewport(0, 0, SHADOW, SHADOW);
        gl.clearDepth(1);
        gl.clear(gl.DEPTH_BUFFER_BIT);
        gl.disable(gl.CULL_FACE);
        gl.enable(gl.POLYGON_OFFSET_FILL);
        gl.polygonOffset(1.75, 1);
        gl.useProgram(shade.p);
        gl.uniformMatrix4fv(shade.u.lightVP, false, lightVP);
        nodes.forEach((n, i) => {
          const m = n.cast && mesh(n.mesh);
          if (!m) return;
          gl.uniformMatrix4fv(shade.u.model, false, placed[i]);
          gl.bindVertexArray(m.vao);
          gl.drawElements(gl.TRIANGLES, m.count, gl.UNSIGNED_INT, 0);
        });
        gl.disable(gl.POLYGON_OFFSET_FILL);
        gl.bindFramebuffer(gl.FRAMEBUFFER, null);
        map = shadowMap;
      }
      gl.viewport(0, 0, w, h);
      const bg = props.background >>> 0;
      gl.clearColor((bg >>> 24) / 255, ((bg >>> 16) & 255) / 255, ((bg >>> 8) & 255) / 255, (bg & 255) / 255);
      gl.clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT);
      gl.enable(gl.BLEND);
      gl.blendFuncSeparate(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA, gl.ONE, gl.ONE_MINUS_SRC_ALPHA);
      gl.frontFace(gl.CCW);
      gl.useProgram(main.p);
      const u = main.u;
      gl.uniformMatrix4fv(u.vp, false, mul(projection(props.fov, w / h, props.near, props.far), lookAt(eye, target)));
      gl.uniformMatrix4fv(u.lightVP, false, lightVP);
      gl.uniform4f(u.eye, eye[0], eye[1], eye[2], 1);
      gl.uniform4f(u.ambient, props.ambient, shadows ? 1 : 0, 0, 0);
      const lights = (props.lights || []).slice(0, 4), dirs = new Float32Array(16), colors = new Float32Array(16);
      lights.forEach((l, i) => { dirs.set([l.direction.X, l.direction.Y, l.direction.Z, 0], i * 4); colors.set([...(l.color || [1, 1, 1]), 0], i * 4); });
      gl.uniform4fv(u.dirs, dirs);
      gl.uniform4fv(u.colors, colors);
      gl.uniform1i(u.count, lights.length);
      gl.uniform1i(u.tex, 0);
      gl.uniform1i(u.map, 1);
      gl.activeTexture(gl.TEXTURE1);
      gl.bindTexture(gl.TEXTURE_2D, map);
      nodes.forEach((n, i) => {
        const m = mesh(n.mesh);
        if (!m) return;
        if (n.double) gl.disable(gl.CULL_FACE); else { gl.enable(gl.CULL_FACE); gl.cullFace(gl.BACK); }
        gl.activeTexture(gl.TEXTURE0);
        gl.bindTexture(gl.TEXTURE_2D, texture(n.texture));
        gl.uniformMatrix4fv(u.model, false, placed[i]);
        gl.uniformMatrix4fv(u.normalM, false, invertTranspose(placed[i]));
        gl.uniform4fv(u.color, n.color || [1, 1, 1, 1]);
        gl.uniform4f(u.params, n.shine || 0, n.unlit ? 1 : 0, 0, 0);
        gl.bindVertexArray(m.vao);
        gl.drawElements(gl.TRIANGLES, m.count, gl.UNSIGNED_INT, 0);
      });
      gl.bindVertexArray(null);
      if (nodes.some((n) => n.speed)) frame = requestAnimationFrame(draw);
    }
    new ResizeObserver(redraw).observe(el);

    return (p) => {
      props = p;
      asked = new Set();
      // Go's camera when it changes; Orbit's otherwise.
      const cam = JSON.stringify([p.eye, p.target]);
      if (cam !== goCamera) {
        goCamera = cam;
        eye = [p.eye.X, p.eye.Y, p.eye.Z];
        target = [p.target.X, p.target.Y, p.target.Z];
        const d = [eye[0] - target[0], eye[1] - target[1], eye[2] - target[2]];
        dist = Math.max(Math.hypot(...d), 1e-3);
        yaw = Math.atan2(d[0], d[2]) * 180 / Math.PI;
        pitch = Math.asin(Math.min(1, Math.max(-1, d[1] / dist))) * 180 / Math.PI;
      }
      nodes = p.nodes || [];
      // A node keeps turning from where it was across updates.
      const ids = new Set(nodes.map((n) => n.id)), t = performance.now() / 1000;
      for (const id of [...started.keys()]) if (!ids.has(id)) started.delete(id);
      for (const n of nodes) if (n.speed && !started.has(n.id)) started.set(n.id, t);
      host.use();
    };
  });
})();
