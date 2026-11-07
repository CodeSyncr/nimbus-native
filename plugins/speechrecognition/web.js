// The Web Speech API: Chrome sends the audio to Google, Safari to Apple.
(() => {
  let rec = null, text = "", done = true, err = null;
  const Rec = () => window.SpeechRecognition || window.webkitSpeechRecognition || NN.fail("unavailable", "this browser can't recognise speech");
  NN.plugin("speechrecognition", {
    request: async () => {
      Rec();
      try { (await navigator.mediaDevices.getUserMedia({ audio: true })).getTracks().forEach((t) => t.stop()); return { granted: true }; }
      catch (e) { if (e.name === "NotAllowedError") return { granted: false }; throw e; }
    },
    start: ({ language }) => {
      const R = Rec();
      rec?.abort();
      rec = new R();
      if (language) rec.lang = language;
      rec.interimResults = true;
      text = ""; done = false; err = null;
      rec.onresult = (e) => { text = [...e.results].map((r) => r[0].transcript).join(""); };
      rec.onerror = (e) => { if (e.error !== "no-speech" && e.error !== "aborted") err = e; };
      rec.onend = () => { done = true; };
      rec.start();
    },
    read: () => {
      if (err) NN.fail(err.error === "not-allowed" || err.error === "service-not-allowed" ? "denied" : "failed", err.message || err.error);
      return { text: text.trim(), done };
    },
    stop: () => { rec?.stop(); },
  });
})();
