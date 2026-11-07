// Google and Apple in a popup that comes back to the site's /_nn/oauth,
// asking for an ID token directly: no secret, no exchange.
(() => {
  const random = () => [...crypto.getRandomValues(new Uint8Array(16))].map((b) => b.toString(16).padStart(2, "0")).join("");
  async function signIn(page, params) {
    const state = random();
    const q = new URLSearchParams({ ...params, state, redirect_uri: location.origin + "/_nn/oauth" });
    const back = new URL(await NN.popup(page + "?" + q));
    const got = new URLSearchParams(back.hash.slice(1) || back.search.slice(1));
    if (got.get("state") !== state) NN.fail("failed", "the answer wasn't for this sign-in");
    if (got.get("error")) NN.fail(got.get("error") === "user_cancelled_authorize" || got.get("error") === "access_denied" ? "cancelled" : "failed", got.get("error_description") || got.get("error"));
    if (!got.get("id_token")) NN.fail("failed", "no ID token came back");
    return got;
  }
  NN.plugin("signin", {
    google: async ({ nonce, androidClient }) => {
      if (!androidClient) NN.fail("failed", "Google on the web needs Options.AndroidClient, the Web client ID");
      const got = await signIn("https://accounts.google.com/o/oauth2/v2/auth", {
        client_id: androidClient, response_type: "id_token", scope: "openid email profile", nonce: nonce || random(), prompt: "select_account",
      });
      return { idToken: got.get("id_token") };
    },
    apple: async ({ nonce, appleServicesID }) => {
      if (!appleServicesID) NN.fail("unavailable", "Sign in with Apple on the web needs Options.AppleServicesID");
      const params = { client_id: appleServicesID, response_type: "code id_token", response_mode: "form_post", scope: "name email" };
      if (nonce) params.nonce = nonce;
      const got = await signIn("https://appleid.apple.com/auth/authorize", params);
      let name = "";
      try { const n = JSON.parse(got.get("user") || "{}").name || {}; name = [n.firstName, n.lastName].filter(Boolean).join(" "); } catch (_) {}
      return { idToken: got.get("id_token"), code: got.get("code") || "", name };
    },
  });
})();
