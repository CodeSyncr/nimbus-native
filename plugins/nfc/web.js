// Web NFC, in Chrome on Android.
NN.plugin("nfc", {
  available: () => ({ available: "NDEFReader" in window }),
  read: async () => {
    if (!("NDEFReader" in window)) NN.fail("unavailable", "this browser can't read NFC");
    const reader = new NDEFReader(), stop = new AbortController();
    await reader.scan({ signal: stop.signal });
    const e = await new Promise((ok, no) => { reader.onreading = ok; reader.onreadingerror = () => no(new Error("couldn't read the tag")); });
    stop.abort();
    const records = [];
    for (const r of e.message.records) {
      if (r.recordType === "text" || r.recordType === "url") records.push(new TextDecoder(r.encoding || "utf-8").decode(r.data));
    }
    return { id: (e.serialNumber || "").replaceAll(":", ""), records };
  },
});
