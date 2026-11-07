// MapKit JS, loaded once and started with the token Go sends
// (applemaps.WebToken). Zoom is Google's, turned into a span as on iOS.
(() => {
  let loading = null;
  const load = (token) => loading ||= new Promise((ok, no) => {
    const s = document.createElement("script");
    s.src = "https://cdn.apple-mapkit.com/mk/5.x.x/mapkit.core.js";
    s.crossOrigin = "anonymous";
    s.dataset.libraries = "map,annotations,overlays";
    s.dataset.callback = "__nnMapKit";
    window.__nnMapKit = () => { mapkit.init({ authorizationCallback: (done) => done(token) }); ok(); };
    s.onerror = () => { loading = null; no(new Error("MapKit JS didn't load")); };
    document.head.appendChild(s);
  });
  const rgba = (c) => { c >>>= 0; return "rgba(" + (c >>> 24) + "," + ((c >>> 16) & 255) + "," + ((c >>> 8) & 255) + "," + ((c & 255) / 255) + ")"; };
  NN.view("applemaps", (el, emit) => {
    let map = null, camera = "", pending = null, ours = false;
    const width = () => el.getBoundingClientRect().width || 320;
    function region(at, z) {
      const lng = Math.min(360, 360 / 2 ** z * width() / 256);
      const r = el.getBoundingClientRect(), tall = (r.height || r.width) / Math.max(1, r.width);
      const lat = Math.min(170, lng * tall * Math.cos(at.lat * Math.PI / 180));
      return new mapkit.CoordinateRegion(new mapkit.Coordinate(at.lat, at.lng), new mapkit.CoordinateSpan(lat, lng));
    }
    function apply(p) {
      if (!map) {
        map = new mapkit.Map(el, { showsCompass: mapkit.FeatureVisibility.Hidden, showsMapTypeControl: false });
        map.addEventListener("single-tap", (e) => { const c = map.convertPointOnPageToCoordinate(e.pointOnPage); emit("tap", { at: { lat: c.latitude, lng: c.longitude } }); });
        map.addEventListener("select", (e) => { if (e.annotation?.data?.id) emit("marker", { id: e.annotation.data.id }); });
        map.addEventListener("region-change-end", () => {
          if (ours) { ours = false; return; }
          const r = map.region, z = Math.log2(360 * width() / 256 / Math.max(r.span.longitudeDelta, 1e-9));
          emit("move", { at: { lat: r.center.latitude, lng: r.center.longitude }, zoom: z });
        });
      }
      const cam = JSON.stringify([p.center, p.zoom]);
      if (cam !== camera) { camera = cam; ours = true; map.setRegionAnimated(region(p.center, p.zoom), true); }
      map.mapType = { satellite: mapkit.Map.MapTypes.Satellite, hybrid: mapkit.Map.MapTypes.Hybrid }[p.kind] || mapkit.Map.MapTypes.Standard;
      map.showsUserLocation = !!p.myLocation;
      map.removeAnnotations(map.annotations);
      map.addAnnotations(p.markers.map((m) => new mapkit.MarkerAnnotation(new mapkit.Coordinate(m.at.lat, m.at.lng), { title: m.title || "", subtitle: m.subtitle || "", data: { id: m.id } })));
      map.removeOverlays(map.overlays);
      map.addOverlays(p.lines.map((l) => new mapkit.PolylineOverlay(l.points.map((q) => new mapkit.Coordinate(q.lat, q.lng)), { style: new mapkit.Style({ strokeColor: rgba(l.color), lineWidth: l.width }) })));
    }
    return (p) => {
      if (!p.token) return;
      pending = p;
      load(p.token).then(() => { if (pending) { const q = pending; pending = null; apply(q); } }, () => {});
    };
  });
})();
