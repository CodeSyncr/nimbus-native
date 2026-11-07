// The provider's page in a popup, which comes back to the site's /_nn/oauth.
NN.plugin("oauth", {
  origin: () => ({ origin: location.origin }),
  open: async ({ url }) => ({ url: await NN.popup(url) }),
});
