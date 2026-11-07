// What a browser tells about its device, which is less than a phone does.
NN.plugin("device", {
  info: () => {
    const ua = navigator.userAgent, hint = navigator.userAgentData;
    const model = /iPhone|iPad|Android|Macintosh|Windows|Linux|CrOS/.exec(ua)?.[0] || hint?.platform || "";
    return {
      platform: "web", model, manufacturer: model === "iPhone" || model === "iPad" || model === "Macintosh" ? "Apple" : "",
      osVersion: /(?:Android|OS|Windows NT|Mac OS X) ([\d._]+)/.exec(ua)?.[1]?.replaceAll("_", ".") || "",
      memory: Math.round((navigator.deviceMemory || 0) * 2 ** 30), virtual: false,
    };
  },
  battery: async () => {
    const b = await navigator.getBattery?.();
    return b ? { level: b.level, charging: b.charging, lowPower: false } : { level: -1, charging: false, lowPower: false };
  },
});
