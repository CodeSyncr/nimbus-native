// The browser's geolocation: it asks for permission on first use.
(() => {
  let watch = null, latest = null;
  const geo = () => navigator.geolocation || NN.fail("unavailable", "this browser has no location");
  const position = (p) => ({
    latitude: p.coords.latitude, longitude: p.coords.longitude, accuracy: p.coords.accuracy,
    altitude: p.coords.altitude ?? 0, speed: p.coords.speed ?? -1, heading: p.coords.heading ?? -1,
  });
  const once = (o) => new Promise((ok, no) => geo().getCurrentPosition(ok, no, o));
  NN.plugin("geolocation", {
    request: async () => {
      try { await once({ maximumAge: Infinity, timeout: 30000 }); } catch (e) { if (e.code === 1) return { granted: false }; }
      return { granted: true };
    },
    current: async () => {
      try { return position(await once({ enableHighAccuracy: true, timeout: 30000, maximumAge: 0 })); }
      catch (e) { NN.fail(e.code === 1 ? "denied" : "failed", e.message); }
    },
    start: () => {
      if (watch === null) watch = geo().watchPosition((p) => { latest = position(p); }, () => {}, { enableHighAccuracy: true });
    },
    stop: () => {
      if (watch !== null) navigator.geolocation.clearWatch(watch);
      watch = latest = null;
    },
    latest: () => (latest ? { ...latest, known: true } : { known: false }),
  });
})();
