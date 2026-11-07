// Notifications the page shows itself, so only while it is open.
(() => {
  const timers = new Map();
  NN.plugin("localnotifications", {
    request: async () => {
      if (!window.Notification) NN.fail("unavailable", "this browser has no notifications");
      return { granted: (await Notification.requestPermission()) === "granted" };
    },
    schedule: ({ id, title, body, seconds }) => {
      if (!window.Notification) NN.fail("unavailable", "this browser has no notifications");
      if (Notification.permission !== "granted") NN.fail("denied", "notifications aren't allowed");
      clearTimeout(timers.get(id));
      timers.set(id, setTimeout(() => { timers.delete(id); new Notification(title, { body, tag: id }); }, seconds * 1000));
    },
    cancel: ({ id }) => { clearTimeout(timers.get(id)); timers.delete(id); },
  });
})();
