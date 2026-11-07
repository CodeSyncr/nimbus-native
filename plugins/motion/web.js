// Device motion and orientation events, in the phones' units. iOS asks for
// permission, which needs a tap. Computers send no events: reading fails
// with unavailable after a second without one.
(() => {
  const g = 9.80665, rad = Math.PI / 180, apple = /iPhone|iPad/.test(navigator.userAgent);
  const s = { "": { on: null, at: 0, last: null }, "orientation-": { on: null, at: 0, last: null } };
  const events = { "": "devicemotion", "orientation-": "deviceorientation" };
  const read = {
    "": (e) => {
      const a = e.accelerationIncludingGravity || {}, r = e.rotationRate || {}, k = apple ? 1 / g : -1 / g;
      return { accel: { x: (a.x || 0) * k, y: (a.y || 0) * k, z: (a.z || 0) * k },
        gyro: { x: (r.beta || 0) * rad, y: (r.gamma || 0) * rad, z: (r.alpha || 0) * rad } };
    },
    "orientation-": (e) => ({ pitch: e.beta || 0, roll: e.gamma || 0, yaw: e.alpha || 0 }),
  };
  const methods = {};
  for (const p of ["", "orientation-"]) {
    const st = s[p];
    methods[p + "start"] = async () => {
      const Ev = p ? window.DeviceOrientationEvent : window.DeviceMotionEvent;
      if (!Ev) NN.fail("unavailable", "this browser has no motion sensors");
      if (Ev.requestPermission && (await Ev.requestPermission()) !== "granted") NN.fail("denied", "motion wasn't allowed");
      if (!st.on) {
        // A browser with no sensor may still send events, with no values.
        st.on = (e) => { if (p ? e.beta != null : e.accelerationIncludingGravity?.x != null) st.last = read[p](e); };
        addEventListener(events[p], st.on);
      }
      st.at = performance.now();
    };
    methods[p + "stop"] = () => {
      if (st.on) removeEventListener(events[p], st.on);
      st.on = st.last = null;
    };
    methods[p + "read"] = () => {
      if (!st.last) {
        if (performance.now() - st.at > 1000) NN.fail("unavailable", "no motion sensor here");
        return p ? { pitch: 0, roll: 0, yaw: 0 } : { accel: { x: 0, y: 0, z: 0 }, gyro: { x: 0, y: 0, z: 0 } };
      }
      return st.last;
    };
  }
  NN.plugin("motion", methods);
})();
