// The site's service worker: it shows push notifications while no tab is
// open, and brings the site back when one is tapped. Open tabs hear of each
// message too, for push.Watch.
self.addEventListener("install", () => self.skipWaiting());
self.addEventListener("activate", (e) => e.waitUntil(self.clients.claim()));

self.addEventListener("push", (e) => {
  let m = {};
  try { m = e.data ? e.data.json() : {}; } catch (_) { m = { body: e.data.text() }; }
  e.waitUntil((async () => {
    for (const c of await self.clients.matchAll({ type: "window" })) c.postMessage({ nnPush: { ...m, tapped: false } });
    // Every push shows a notification: browsers require it.
    await self.registration.showNotification(m.title || "", { body: m.body || "", data: m, tag: m.tag || undefined });
  })());
});

self.addEventListener("notificationclick", (e) => {
  e.notification.close();
  const m = e.notification.data || {};
  const url = new URL(m.url || "/", self.location.origin).href;
  e.waitUntil((async () => {
    const open = await self.clients.matchAll({ type: "window", includeUncontrolled: true });
    let c = open.find((c) => c.url === url) || open[0];
    if (c) c = await c.focus().catch(() => c);
    else c = await self.clients.openWindow(url);
    c?.postMessage({ nnPush: { ...m, tapped: true } });
  })());
});
