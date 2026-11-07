// The Contact Picker API, in Chrome on Android. Browsers can't list, add
// or change contacts.
NN.plugin("contacts", {
  pick: async () => {
    if (!navigator.contacts?.select) NN.fail("unavailable", "this browser can't pick contacts");
    const [c] = await navigator.contacts.select(["name", "tel", "email"]);
    if (!c) NN.fail("cancelled", "no contact was chosen");
    return { id: "", name: c.name?.[0] || "", phones: c.tel || [], emails: c.email || [] };
  },
});
