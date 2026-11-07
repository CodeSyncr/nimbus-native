// In a browser the app is "in front" while its tab is showing.
NN.plugin("appstate", {
  get: () => ({ active: document.visibilityState === "visible" }),
});
