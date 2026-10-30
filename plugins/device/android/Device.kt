package space.nimbusgo.nativekit

import android.app.ActivityManager
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.os.BatteryManager
import android.os.Build

/** device: the phone's model, system, memory and battery. */
internal object DevicePlugin {
    fun handle(call: PluginCall) {
        val c = Plugins.context
        when (call.method) {
            "info" -> {
                val memory = ActivityManager.MemoryInfo()
                (c.getSystemService(Context.ACTIVITY_SERVICE) as ActivityManager).getMemoryInfo(memory)
                val f = Build.FINGERPRINT
                call.resolve(mapOf("platform" to "android", "model" to Build.MODEL, "manufacturer" to Build.MANUFACTURER,
                    "osVersion" to Build.VERSION.RELEASE, "memory" to memory.totalMem,
                    "virtual" to (f.startsWith("generic") || f.contains("emulator") || Build.HARDWARE.startsWith("ranchu"))))
            }
            "battery" -> {
                val s = c.registerReceiver(null, IntentFilter(Intent.ACTION_BATTERY_CHANGED))
                val level = s?.getIntExtra(BatteryManager.EXTRA_LEVEL, -1) ?: -1
                val scale = s?.getIntExtra(BatteryManager.EXTRA_SCALE, -1) ?: -1
                val status = s?.getIntExtra(BatteryManager.EXTRA_STATUS, -1) ?: -1
                call.resolve(mapOf("level" to (if (scale > 0 && level >= 0) level.toFloat() / scale else -1f),
                    "charging" to (status == BatteryManager.BATTERY_STATUS_CHARGING || status == BatteryManager.BATTERY_STATUS_FULL)))
            }
            else -> Plugins.unknownMethod(call)
        }
    }
}
