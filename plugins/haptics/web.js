// The Vibration API: Chrome on Android buzzes; iPhones and computers don't.
(() => {
  const buzz = (p) => { navigator.vibrate?.(p); };
  NN.plugin("haptics", {
    impact: ({ style, intensity }) => buzz(Math.round(({ light: 8, soft: 8, medium: 14, rigid: 16, heavy: 22 }[style] || 14) * (0.5 + 0.5 * (intensity ?? 1)))),
    notify: ({ kind }) => buzz({ success: [12, 60, 18], warning: [20, 80, 20], error: [25, 60, 25, 60, 25] }[kind] || 15),
    selection: () => buzz(5),
    play: ({ events = [] }) => {
      // Taps and buzzes in time order, as on/off pairs; strength can't vary.
      const p = [];
      let at = 0;
      for (const e of [...events].sort((a, b) => a.T - b.T)) {
        const start = Math.round(e.T * 1000), len = Math.max(Math.round(e.D * 1000), 10);
        if (start < at) continue;
        p.push(start - at, len);
        at = start + len;
      }
      if (p.length) buzz(p[0] === 0 ? p.slice(1) : [0, ...p]);
    },
  });
})();
