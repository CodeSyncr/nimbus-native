package space.nimbusgo.nativekit

import android.Manifest
import android.content.Context
import android.content.pm.PackageManager
import android.hardware.Sensor
import android.hardware.SensorEvent
import android.hardware.SensorEventListener
import android.hardware.SensorManager
import android.os.Build

/** pedometer: the step counter, which counts from boot, so steps are from the first event. */
internal object PedometerPlugin {
    private var base = -1f
    private var steps = 0
    private val manager get() = Plugins.context.getSystemService(Context.SENSOR_SERVICE) as SensorManager
    private val sensor get() = manager.getDefaultSensor(Sensor.TYPE_STEP_COUNTER)

    private val listener = object : SensorEventListener {
        override fun onSensorChanged(e: SensorEvent) {
            if (base < 0) base = e.values[0]
            steps = (e.values[0] - base).toInt()
        }

        override fun onAccuracyChanged(s: Sensor?, a: Int) {}
    }

    private fun allowed() = Build.VERSION.SDK_INT < 29 ||
        Plugins.context.checkSelfPermission(Manifest.permission.ACTIVITY_RECOGNITION) == PackageManager.PERMISSION_GRANTED

    fun handle(call: PluginCall) {
        when (call.method) {
            "available" -> call.resolve(mapOf("available" to (sensor != null)))
            "request" -> {
                if (sensor == null) return call.reject(Plugins.UNAVAILABLE, "this phone can't count steps")
                if (allowed()) return call.resolve(mapOf("granted" to true))
                val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no activity to ask from")
                PermissionAsker.ask(a, Manifest.permission.ACTIVITY_RECOGNITION) { call.resolve(mapOf("granted" to it)) }
            }
            "start" -> {
                val s = sensor ?: return call.reject(Plugins.UNAVAILABLE, "this phone can't count steps")
                if (!allowed()) return call.reject(Plugins.DENIED, "steps aren't allowed: call Request first")
                base = -1f
                steps = 0
                manager.registerListener(listener, s, SensorManager.SENSOR_DELAY_NORMAL)
                call.resolve()
            }
            "read" -> call.resolve(mapOf("steps" to steps))
            "stop" -> {
                manager.unregisterListener(listener)
                call.resolve()
            }
            "between" -> call.reject(Plugins.UNAVAILABLE, "Android keeps no history of steps")
            else -> Plugins.unknownMethod(call)
        }
    }
}
