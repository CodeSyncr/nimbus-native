package space.nimbusgo.nativekit

import android.content.Context
import android.hardware.Sensor
import android.hardware.SensorEvent
import android.hardware.SensorEventListener
import android.hardware.SensorManager

/** barometer: the pressure sensor; height is worked out from the first reading, read as Go asks. */
internal object BarometerPlugin {
    private val manager get() = Plugins.context.getSystemService(Context.SENSOR_SERVICE) as SensorManager
    private var pressure = 0f
    private var first = 0f

    private val listener = object : SensorEventListener {
        override fun onSensorChanged(e: SensorEvent) {
            pressure = e.values[0]
            if (first == 0f) first = pressure
        }

        override fun onAccuracyChanged(s: Sensor?, a: Int) {}
    }

    private fun started() { pressure = 0f; first = 0f }

    private fun altitude() = if (first == 0f) 0f else
        SensorManager.getAltitude(first, pressure)

    fun handle(call: PluginCall) {
        when (call.method) {
            "available" -> call.resolve(mapOf("available" to (manager.getDefaultSensor(Sensor.TYPE_PRESSURE) != null)))
            "start" -> {
                val s = manager.getDefaultSensor(Sensor.TYPE_PRESSURE) ?: return call.reject(Plugins.UNAVAILABLE, "this phone has no such sensor")
                val hz = call.args.optInt("hz", 1).coerceAtLeast(1)
                started()
                manager.registerListener(listener, s, 1_000_000 / hz)
                call.resolve()
            }
            "read" -> call.resolve(mapOf("pressure" to pressure.toDouble(), "altitude" to altitude().toDouble()))
            "stop" -> {
                manager.unregisterListener(listener)
                call.resolve()
            }
            else -> Plugins.unknownMethod(call)
        }
    }
}
