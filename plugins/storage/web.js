// The visitor's own browser storage.
NN.plugin("storage", {
  get: ({ key }) => {
    const v = localStorage.getItem("nn.storage." + key);
    return v === null ? { found: false } : { found: true, value: JSON.parse(v) };
  },
  set: ({ key, value }) => { localStorage.setItem("nn.storage." + key, JSON.stringify(value)); },
  delete: ({ key }) => { localStorage.removeItem("nn.storage." + key); },
});
