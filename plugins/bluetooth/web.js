// Web Bluetooth (Chrome and Edge). A browser doesn't scan in the background:
// Scan opens its chooser, and the device picked is the one found. Only the
// services named in Scan can be read or written afterwards.
(() => {
  const devices = new Map(); // id → BluetoothDevice
  let found = [];
  // Phones take "180D"; browsers want the full lower-case UUID.
  const uuid = (s) => (/^[0-9a-f]{4}$/i.test(s) ? "0000" + s + "-0000-1000-8000-00805f9b34fb" : s).toLowerCase();
  const ble = () => navigator.bluetooth || NN.fail("unavailable", "this browser has no Bluetooth (Chrome and Edge do)");
  const device = (id) => devices.get(id) || NN.fail("not_found", "scan for it first");
  const characteristic = async ({ id, service, characteristic }) => {
    const d = device(id);
    if (!d.gatt.connected) NN.fail("failed", "connect first");
    return (await d.gatt.getPrimaryService(uuid(service))).getCharacteristic(uuid(characteristic));
  };
  NN.plugin("bluetooth", {
    request: async () => ({ granted: await ble().getAvailability() }),
    scanStart: async ({ services }) => {
      const want = (services || []).map(uuid);
      const d = await ble().requestDevice(want.length ? { filters: [{ services: want }] } : { acceptAllDevices: true });
      devices.set(d.id, d);
      found.push({ id: d.id, name: d.name || "", signal: 0 });
    },
    scanRead: () => ({ devices: found.splice(0) }),
    scanStop: () => {},
    connect: async ({ id }) => { await device(id).gatt.connect(); },
    disconnect: ({ id }) => { device(id).gatt.disconnect(); },
    read: async (a) => {
      const v = await (await characteristic(a)).readValue();
      return { value: await NN.base64(new Uint8Array(v.buffer, v.byteOffset, v.byteLength)) };
    },
    write: async (a) => { await (await characteristic(a)).writeValueWithResponse(NN.bytes(a.value)); },
  });
})();
