package space.nimbusgo.nativekit

import android.Manifest
import android.annotation.SuppressLint
import android.bluetooth.BluetoothDevice
import android.bluetooth.BluetoothGatt
import android.bluetooth.BluetoothGattCallback
import android.bluetooth.BluetoothGattCharacteristic
import android.bluetooth.BluetoothManager
import android.bluetooth.BluetoothProfile
import android.bluetooth.le.ScanCallback
import android.bluetooth.le.ScanFilter
import android.bluetooth.le.ScanResult
import android.bluetooth.le.ScanSettings
import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import android.os.ParcelUuid
import android.util.Base64
import org.json.JSONArray
import org.json.JSONObject
import java.util.UUID

/** bluetooth: the platform's LE central. One operation at a time per device, as GATT requires. */
@SuppressLint("MissingPermission") // request comes first; the system still checks
internal object BluetoothPlugin {
    private val found = HashMap<String, BluetoothDevice>()
    private val gatts = HashMap<String, BluetoothGatt>()
    private val waiting = HashMap<String, PluginCall>()
    private var inbox = JSONArray()

    private val manager get() = Plugins.context.getSystemService(Context.BLUETOOTH_SERVICE) as BluetoothManager
    private val needed get() = if (Build.VERSION.SDK_INT >= 31) listOf(Manifest.permission.BLUETOOTH_SCAN, Manifest.permission.BLUETOOTH_CONNECT) else listOf(Manifest.permission.ACCESS_FINE_LOCATION)

    private fun allowed() = needed.all { Plugins.context.checkSelfPermission(it) == PackageManager.PERMISSION_GRANTED }

    private val scan = object : ScanCallback() {
        override fun onScanResult(type: Int, r: ScanResult) {
            synchronized(this@BluetoothPlugin) {
                found[r.device.address] = r.device
                inbox.put(JSONObject().put("id", r.device.address).put("name", r.device.name ?: "").put("signal", r.rssi))
            }
        }
    }

    private val callback = object : BluetoothGattCallback() {
        override fun onConnectionStateChange(g: BluetoothGatt, status: Int, state: Int) {
            val id = g.device.address
            if (state == BluetoothProfile.STATE_CONNECTED) {
                gatts[id] = g
                g.discoverServices()
            } else {
                gatts.remove(id)
                g.close()
                take("connect$id")?.reject(Plugins.FAILED, "couldn't connect ($status)")
            }
        }

        override fun onServicesDiscovered(g: BluetoothGatt, status: Int) {
            take("connect${g.device.address}")?.resolve()
        }

        @Deprecated("Deprecated in Java")
        override fun onCharacteristicRead(g: BluetoothGatt, x: BluetoothGattCharacteristic, status: Int) {
            val c = take("r${g.device.address}${x.uuid}") ?: return
            @Suppress("DEPRECATION")
            if (status == BluetoothGatt.GATT_SUCCESS) c.resolve(mapOf("value" to Base64.encodeToString(x.value ?: ByteArray(0), Base64.NO_WRAP)))
            else c.reject(Plugins.FAILED, "the read failed ($status)")
        }

        override fun onCharacteristicWrite(g: BluetoothGatt, x: BluetoothGattCharacteristic, status: Int) {
            val c = take("w${g.device.address}${x.uuid}") ?: return
            if (status == BluetoothGatt.GATT_SUCCESS) c.resolve() else c.reject(Plugins.FAILED, "the write failed ($status)")
        }
    }

    @Synchronized private fun take(k: String) = waiting.remove(k)
    @Synchronized private fun hold(k: String, c: PluginCall) { waiting[k] = c }

    private fun characteristic(c: PluginCall): Pair<BluetoothGatt, BluetoothGattCharacteristic>? {
        val g = gatts[c.string("id")] ?: return null
        val x = g.getService(UUID.fromString(c.string("service")))?.getCharacteristic(UUID.fromString(c.string("characteristic"))) ?: return null
        return g to x
    }

    fun handle(call: PluginCall) {
        when (call.method) {
            "request" -> {
                if (allowed()) return call.resolve(mapOf("granted" to true))
                val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no activity to ask from")
                val next = needed.firstOrNull { Plugins.context.checkSelfPermission(it) != PackageManager.PERMISSION_GRANTED }!!
                PermissionAsker.ask(a, next) { ok ->
                    if (!ok || allowed()) call.resolve(mapOf("granted" to ok))
                    else PermissionAsker.ask(a, needed.last()) { call.resolve(mapOf("granted" to it)) }
                }
            }
            "scanStart" -> {
                val s = manager.adapter?.bluetoothLeScanner ?: return call.reject(Plugins.UNAVAILABLE, "Bluetooth is off or missing")
                if (!allowed()) return call.reject(Plugins.DENIED, "Bluetooth is not allowed")
                val list = call.args.optJSONArray("services")
                val filters = (0 until (list?.length() ?: 0)).map { ScanFilter.Builder().setServiceUuid(ParcelUuid.fromString(list!!.getString(it))).build() }
                s.startScan(filters, ScanSettings.Builder().setScanMode(ScanSettings.SCAN_MODE_LOW_LATENCY).build(), scan)
                call.resolve()
            }
            "scanRead" -> synchronized(this) {
                call.resolve(JSONObject().put("devices", inbox))
                inbox = JSONArray()
            }
            "scanStop" -> {
                manager.adapter?.bluetoothLeScanner?.stopScan(scan)
                call.resolve()
            }
            "connect" -> {
                val id = call.string("id") ?: ""
                val d = found[id] ?: return call.reject(Plugins.NOT_FOUND, "scan for it first")
                hold("connect$id", call)
                d.connectGatt(Plugins.context, false, callback, BluetoothDevice.TRANSPORT_LE)
            }
            "disconnect" -> {
                gatts[call.string("id")]?.disconnect()
                call.resolve()
            }
            "read" -> {
                val (g, x) = characteristic(call) ?: return call.reject(Plugins.NOT_FOUND, "no such characteristic")
                hold("r${g.device.address}${x.uuid}", call)
                g.readCharacteristic(x)
            }
            "write" -> {
                val (g, x) = characteristic(call) ?: return call.reject(Plugins.NOT_FOUND, "no such characteristic")
                hold("w${g.device.address}${x.uuid}", call)
                @Suppress("DEPRECATION")
                x.value = Base64.decode(call.string("value") ?: "", Base64.DEFAULT)
                @Suppress("DEPRECATION")
                g.writeCharacteristic(x)
            }
            else -> Plugins.unknownMethod(call)
        }
    }
}
