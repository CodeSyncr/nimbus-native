// Saving downloads the file; a browser can't read the visitor's library.
NN.plugin("medialibrary", {
  request: () => ({ granted: true }),
  save: ({ name, type, data }) => { NN.download(name, type, data); return { id: "" }; },
});
