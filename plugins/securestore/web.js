// Browsers have no keychain: this is the visitor's own browser storage, no
// safer than storage.
NN.plugin("secureStorage", {
  get: ({ key }) => {
    const v = localStorage.getItem("nn.secure." + key);
    return v === null ? { found: false, value: null } : { found: true, value: v };
  },
  set: ({ key, value }) => { localStorage.setItem("nn.secure." + key, value); },
  delete: ({ key }) => { localStorage.removeItem("nn.secure." + key); },
});
