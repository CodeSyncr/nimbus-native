// Web Push: the site's service worker receives what your server sends with
// push.SendWeb, and shows it while no tab is open.
(() => {
  const ready = () => {
    if (!("serviceWorker" in navigator) || !window.PushManager) NN.fail("unavailable", "this browser has no push notifications (on iPhone, add the site to the Home Screen first)");
    return navigator.serviceWorker.register("/_nn/sw.js", { scope: "/" }).then(() => navigator.serviceWorker.ready);
  };
  NN.plugin("push", {
    request: async () => {
      if (!window.Notification) NN.fail("unavailable", "this browser has no notifications");
      await ready();
      return { granted: (await Notification.requestPermission()) === "granted" };
    },
    token: async ({ key }) => {
      const reg = await ready();
      if (Notification.permission !== "granted") NN.fail("denied", "notifications aren't allowed; call push.Request first");
      let sub = await reg.pushManager.getSubscription();
      const want = NN.bytes(key.replace(/-/g, "+").replace(/_/g, "/"));
      const has = sub?.options.applicationServerKey && new Uint8Array(sub.options.applicationServerKey);
      if (sub && (!has || has.length !== want.length || has.some((b, i) => b !== want[i]))) { await sub.unsubscribe(); sub = null; }
      sub ||= await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: want });
      return { value: JSON.stringify(sub), service: "web" };
    },
    messages: () => ({ messages: NN.pushed.splice(0).map((m) => ({ title: m.title || "", body: m.body || "", data: m.data || {}, tapped: !!m.tapped })) }),
  });
})();
