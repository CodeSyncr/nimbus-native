// The browser's languages and Intl's view of the first one. Browsers don't
// say which currency a region uses, so Currency is empty here.
NN.plugin("localization", {
  get: () => {
    const languages = navigator.languages?.length ? [...navigator.languages] : [navigator.language || "en"];
    const locale = new Intl.Locale(languages[0]), full = locale.maximize();
    const region = full.region || "";
    const parts = new Intl.NumberFormat(languages[0]).formatToParts(1000.5);
    const part = (t) => parts.find((p) => p.type === t)?.value || "";
    const week = full.getWeekInfo?.() || full.weekInfo;
    const first = week ? (week.firstDay === 7 ? 1 : week.firstDay + 1) : ["US", "CA", "JP", "IN", "BR", "MX", "IL", "PH"].includes(region) ? 1 : 2;
    const text = full.getTextInfo?.() || full.textInfo;
    const hour = new Intl.DateTimeFormat(languages[0], { hour: "numeric" }).resolvedOptions();
    return {
      languages, language: locale.language, region, currency: "", currencySymbol: "",
      timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone,
      uses24Hour: hour.hourCycle ? hour.hourCycle === "h23" || hour.hourCycle === "h24" : !hour.hour12,
      firstWeekday: first, metric: !["US", "LR", "MM"].includes(region),
      decimalSeparator: part("decimal") || ".", groupingSeparator: part("group"),
      rightToLeft: text ? text.direction === "rtl" : ["ar", "he", "fa", "ur", "ps", "yi", "dv", "ug"].includes(locale.language),
    };
  },
});
