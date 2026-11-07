// The grid Go cut, drawn by WebGL as triangles that blend their corners'
// colors, as on the phones.
NN.view("meshgradient", (el, emit) => {
  const c = document.createElement("canvas");
  c.style.cssText = "width:100%;height:100%;display:block";
  el.appendChild(c);
  const gl = c.getContext("webgl", { premultipliedAlpha: true, antialias: true });
  if (!gl) return () => {};
  const sh = (type, src) => { const s = gl.createShader(type); gl.shaderSource(s, src); gl.compileShader(s); return s; };
  const prog = gl.createProgram();
  gl.attachShader(prog, sh(gl.VERTEX_SHADER, "attribute vec2 p; attribute vec4 c; varying vec4 v; void main() { v = c; gl_Position = vec4(p.x * 2.0 - 1.0, 1.0 - p.y * 2.0, 0.0, 1.0); }"));
  gl.attachShader(prog, sh(gl.FRAGMENT_SHADER, "precision mediump float; varying vec4 v; void main() { gl_FragColor = vec4(v.rgb * v.a, v.a); }"));
  gl.linkProgram(prog);
  gl.useProgram(prog);
  gl.enable(gl.BLEND);
  gl.blendFunc(gl.ONE, gl.ONE_MINUS_SRC_ALPHA);
  const pos = gl.createBuffer(), col = gl.createBuffer(), idx = gl.createBuffer();
  const floats = (b64) => { const b = NN.bytes(b64); return new Float32Array(b.buffer, b.byteOffset, b.byteLength / 4); };
  let count = 0;
  function draw() {
    const r = el.getBoundingClientRect(), k = devicePixelRatio || 1;
    c.width = Math.max(1, Math.round(r.width * k)); c.height = Math.max(1, Math.round(r.height * k));
    gl.viewport(0, 0, c.width, c.height);
    gl.clearColor(0, 0, 0, 0);
    gl.clear(gl.COLOR_BUFFER_BIT);
    if (count) gl.drawElements(gl.TRIANGLES, count, gl.UNSIGNED_SHORT, 0);
  }
  new ResizeObserver(draw).observe(el);
  return ({ w, h, positions, colors }) => {
    if (!w || !h) return;
    const ix = new Uint16Array(w * h * 6);
    let n = 0;
    for (let y = 0; y < h; y++) for (let x = 0; x < w; x++) {
      const a = y * (w + 1) + x, b = a + 1, d = a + w + 1, e = d + 1;
      ix.set([a, d, b, b, d, e], n); n += 6;
    }
    const bind = (buf, data, attr, size) => {
      gl.bindBuffer(gl.ARRAY_BUFFER, buf); gl.bufferData(gl.ARRAY_BUFFER, data, gl.STATIC_DRAW);
      const loc = gl.getAttribLocation(prog, attr); gl.enableVertexAttribArray(loc); gl.vertexAttribPointer(loc, size, gl.FLOAT, false, 0, 0);
    };
    bind(pos, floats(positions), "p", 2);
    bind(col, floats(colors), "c", 4);
    gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, idx); gl.bufferData(gl.ELEMENT_ARRAY_BUFFER, ix, gl.STATIC_DRAW);
    count = ix.length;
    draw();
  };
});
