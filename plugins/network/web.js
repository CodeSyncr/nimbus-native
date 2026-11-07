// The browser's view of the connection; it can't see the device's address.
NN.plugin("network", {
  status: () => {
    const c = navigator.connection, online = navigator.onLine;
    const type = !online ? "none" : { wifi: "wifi", cellular: "cellular", ethernet: "wired" }[c?.type] || "other";
    return { connected: online, type, expensive: !!c?.saveData || type === "cellular", address: "", airplane: false };
  },
});
