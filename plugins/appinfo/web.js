// A site has no bundle, version or folders of its own: the host stands in
// for the ID.
NN.plugin("app", {
  info: () => ({ platform: "web", id: location.host, version: "", build: "", osVersion: "", locale: navigator.language || "", dataDir: "", cacheDir: "" }),
});
