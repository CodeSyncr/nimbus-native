package space.nimbusgo.nativekit

import android.content.Context
import android.hardware.Sensor
import android.hardware.SensorEvent
import android.hardware.SensorEventListener
import android.hardware.SensorManager

/** lightsensor: the ambient light sensor, read as Go asks. */
internal object LightSensorPlugin {
    private val manager get() = Plugins.context.getSystemService(Context.SENSOR_SERVICE) as SensorManager
    private var lux = 0f

    private val listener = object : SensorEventListener {
        override fun onSensorChanged(e: SensorEvent) { lux = e.values[0] }

        override fun onAccuracyChanged(s: Sensor?, a: Int) {}
    }

    private fun started() {}

    fun handle(call: PluginCall) {
        when (call.method) {
            "available" -> call.resolve(mapOf("available" to (manager.getDefaultSensor(Sensor.TYPE_LIGHT) != null)))
            "start" -> {
                val s = manager.getDefaultSensor(Sensor.TYPE_LIGHT) ?: return call.reject(Plugins.UNAVAILABLE, "this phone has no such sensor")
                val hz = call.args.optInt("hz", 2).coerceAtLeast(1)
                started()
                manager.registerListener(listener, s, 1_000_000 / hz)
                call.resolve()
            }
            "read" -> call.resolve(mapOf("lux" to lux.toDouble()))
            "stop" -> {
                manager.unregisterListener(listener)
                call.resolve()
            }
            else -> Plugins.unknownMethod(call)
        }
    }
}
