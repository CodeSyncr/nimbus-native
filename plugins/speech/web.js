// Speech synthesis. Browsers speak only after the page has had a tap.
NN.plugin("speech", {
  say: ({ text, language, rate }) => {
    if (!window.speechSynthesis) NN.fail("unavailable", "this browser can't speak");
    const u = new SpeechSynthesisUtterance(text);
    if (language) u.lang = language;
    u.rate = rate || 1;
    speechSynthesis.speak(u);
  },
  speaking: () => ({ speaking: !!window.speechSynthesis && (speechSynthesis.speaking || speechSynthesis.pending) }),
  stop: () => { window.speechSynthesis?.cancel(); },
});
