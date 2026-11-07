// The web has no such prompt, as on Android.
NN.plugin("trackingconsent", {
  status: () => ({ status: "not-needed" }),
  request: () => ({ status: "not-needed" }),
});
