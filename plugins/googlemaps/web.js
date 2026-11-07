// The Maps JavaScript API, loaded once with the key Go sends
// (googlemaps.WebKey). Pins, lines, the blue dot and the same events as
// the phones.
(() => {
  let loading = null;
  const load = (key) => loading ||= new Promise((ok, no) => {
    window.__nnGoogleMaps = ok;
    const s = document.createElement("script");
    s.src = "https://maps.googleapis.com/maps/api/js?key=" + encodeURIComponent(key) + "&callback=__nnGoogleMaps&loading=async&v=weekly";
    s.async = true;
    s.onerror = () => { loading = null; no(new Error("Google Maps didn't load")); };
    document.head.appendChild(s);
  });
  const rgba = (c) => { c >>>= 0; return ["rgb(" + (c >>> 24) + "," + ((c >>> 16) & 255) + "," + ((c >>> 8) & 255) + ")", (c & 255) / 255]; };
  NN.view("googlemaps", (el, emit) => {
    let map = null, pins = [], lines = [], dot = null, watch = null, camera = "", pending = null, moving = false;
    const ll = (p) => ({ lat: p.lat, lng: p.lng });
    function apply(p) {
      const g = google.maps;
      if (!map) {
        map = new g.Map(el, { center: ll(p.center), zoom: p.zoom, disableDefaultUI: true, zoomControl: true, clickableIcons: false, gestureHandling: "greedy" });
        map.addListener("click", (e) => emit("tap", { at: { lat: e.latLng.lat(), lng: e.latLng.lng() } }));
        // A move is the person's: a pan or zoom Go didn't ask for.
        map.addListener("dragstart", () => { moving = true; });
        map.addListener("zoom_changed", () => { if (!camera.startsWith("go:")) moving = true; });
        map.addListener("idle", () => {
          if (camera.startsWith("go:")) { camera = camera.slice(3); return; }
          if (!moving) return;
          moving = false;
          const c = map.getCenter();
          emit("move", { at: { lat: c.lat(), lng: c.lng() }, zoom: map.getZoom() });
        });
      }
      const cam = JSON.stringify([p.center, p.zoom]);
      if (cam !== camera.replace(/^go:/, "")) { camera = "go:" + cam; map.setZoom(p.zoom); map.panTo(ll(p.center)); }
      pins.forEach((m) => m.setMap(null));
      pins = p.markers.map((m) => {
        const pin = new g.Marker({ map, position: ll(m.at), title: m.title || "" });
        pin.addListener("click", () => emit("marker", { id: m.id }));
        return pin;
      });
      lines.forEach((l) => l.setMap(null));
      lines = p.lines.map((l) => { const [color, opacity] = rgba(l.color); return new g.Polyline({ map, path: l.points.map(ll), strokeColor: color, strokeOpacity: opacity, strokeWeight: l.width }); });
      if (p.myLocation && watch === null && navigator.geolocation) {
        watch = navigator.geolocation.watchPosition((pos) => {
          const at = { lat: pos.coords.latitude, lng: pos.coords.longitude };
          dot ||= new g.Marker({ map, clickable: false, icon: { path: g.SymbolPath.CIRCLE, scale: 7, fillColor: "#1a73e8", fillOpacity: 1, strokeColor: "#fff", strokeWeight: 2 } });
          dot.setPosition(at);
        }, () => {});
      } else if (!p.myLocation && watch !== null) {
        navigator.geolocation.clearWatch(watch); watch = null; dot?.setMap(null); dot = null;
      }
    }
    return (p) => {
      if (!p.key) return; // no key: an empty box, as the docs say
      pending = p;
      load(p.key).then(() => { if (pending) { const q = pending; pending = null; apply(q); } }, () => {});
    };
  });
})();
