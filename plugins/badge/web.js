// The Badging API, which shows only for an installed web app.
NN.plugin("badge", {
  set: async ({ count }) => {
    if (!navigator.setAppBadge) NN.fail("unavailable", "this browser has no app badge");
    if (count > 0) await navigator.setAppBadge(count);
    else await navigator.clearAppBadge();
  },
});
