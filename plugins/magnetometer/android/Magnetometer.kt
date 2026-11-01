package space.nimbusgo.nativekit

import android.content.Context
import android.hardware.Sensor
import android.hardware.SensorEvent
import android.hardware.SensorEventListener
import android.hardware.SensorManager

/** magnetometer: the raw field, in microteslas as on iOS, read as Go asks. */
internal object MagnetometerPlugin {
    private val manager get() = Plugins.context.getSystemService(Context.SENSOR_SERVICE) as SensorManager
    private val field = FloatArray(3)

    private val listener = object : SensorEventListener {
        override fun onSensorChanged(e: SensorEvent) { e.values.copyInto(field, 0, 0, 3) }

        override fun onAccuracyChanged(s: Sensor?, a: Int) {}
    }

    private fun started() {}

    fun handle(call: PluginCall) {
        when (call.method) {
            "available" -> call.resolve(mapOf("available" to (manager.getDefaultSensor(Sensor.TYPE_MAGNETIC_FIELD) != null)))
            "start" -> {
                val s = manager.getDefaultSensor(Sensor.TYPE_MAGNETIC_FIELD) ?: return call.reject(Plugins.UNAVAILABLE, "this phone has no such sensor")
                val hz = call.args.optInt("hz", 10).coerceAtLeast(1)
                started()
                manager.registerListener(listener, s, 1_000_000 / hz)
                call.resolve()
            }
            "read" -> call.resolve(mapOf("x" to field[0].toDouble(), "y" to field[1].toDouble(), "z" to field[2].toDouble()))
            "stop" -> {
                manager.unregisterListener(listener)
                call.resolve()
            }
            else -> Plugins.unknownMethod(call)
        }
    }
}
