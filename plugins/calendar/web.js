// An .ics file, which the visitor's calendar opens to add the event. Like
// Android, it can't tell whether they saved it.
NN.plugin("calendar", {
  add: async ({ title, location, notes, start, end }) => {
    const at = (s) => new Date(s * 1000).toISOString().replace(/[-:]/g, "").replace(/\.\d+/, "");
    const esc = (s) => (s || "").replace(/[\;,]/g, (c) => "\\" + c).replace(/\n/g, "\\n");
    const ics = ["BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//Nimbus Native//EN", "BEGIN:VEVENT",
      "UID:" + crypto.randomUUID() + "@" + window.location.host, "DTSTAMP:" + at(Date.now() / 1000),
      "DTSTART:" + at(start), "DTEND:" + at(end), "SUMMARY:" + esc(title),
      location ? "LOCATION:" + esc(location) : "", notes ? "DESCRIPTION:" + esc(notes) : "",
      "END:VEVENT", "END:VCALENDAR"].filter(Boolean).join("\r\n");
    NN.download((title || "event").replace(/[^\w -]+/g, "") + ".ics", "text/calendar", await NN.base64(new TextEncoder().encode(ics)));
    return { saved: true };
  },
});
