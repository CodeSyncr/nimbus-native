package space.nimbusgo.nativekit

import android.content.Context
import android.hardware.Sensor
import android.hardware.SensorEvent
import android.hardware.SensorEventListener
import android.hardware.SensorManager
import org.json.JSONObject

/** motion: the sensors, read as Go asks. Acceleration comes in g, as on iOS. */
internal object MotionPlugin {
    private val accel = FloatArray(3)
    private val gyro = FloatArray(3)
    private val manager get() = Plugins.context.getSystemService(Context.SENSOR_SERVICE) as SensorManager

    private val listener = object : SensorEventListener {
        override fun onSensorChanged(e: SensorEvent) {
            when (e.sensor.type) {
                Sensor.TYPE_ACCELEROMETER -> for (i in 0..2) accel[i] = e.values[i] / -SensorManager.GRAVITY_EARTH
                Sensor.TYPE_GYROSCOPE -> e.values.copyInto(gyro, 0, 0, 3)
            }
        }

        override fun onAccuracyChanged(s: Sensor?, a: Int) {}
    }

    private val matrix = FloatArray(9).also { it[0] = 1f; it[4] = 1f; it[8] = 1f }
    private val gravity = FloatArray(3)
    private val field = FloatArray(3)

    /** The rotation sensor gives the turn directly; without one it's worked out from gravity and north. */
    private val turned = object : SensorEventListener {
        override fun onSensorChanged(e: SensorEvent) {
            when (e.sensor.type) {
                Sensor.TYPE_ROTATION_VECTOR -> SensorManager.getRotationMatrixFromVector(matrix, e.values)
                Sensor.TYPE_ACCELEROMETER -> e.values.copyInto(gravity, 0, 0, 3)
                Sensor.TYPE_MAGNETIC_FIELD -> e.values.copyInto(field, 0, 0, 3)
            }
            if (e.sensor.type != Sensor.TYPE_ROTATION_VECTOR) SensorManager.getRotationMatrix(matrix, null, gravity, field)
        }

        override fun onAccuracyChanged(s: Sensor?, a: Int) {}
    }

    private fun vector(v: FloatArray) = JSONObject().put("x", v[0].toDouble()).put("y", v[1].toDouble()).put("z", v[2].toDouble())

    fun handle(call: PluginCall) {
        when (call.method) {
            "start" -> {
                val a = manager.getDefaultSensor(Sensor.TYPE_ACCELEROMETER) ?: return call.reject(Plugins.UNAVAILABLE, "this phone has no motion sensors")
                val us = 1_000_000 / call.args.optInt("hz", 30).coerceAtLeast(1)
                manager.registerListener(listener, a, us)
                manager.getDefaultSensor(Sensor.TYPE_GYROSCOPE)?.let { manager.registerListener(listener, it, us) }
                call.resolve()
            }
            "read" -> call.resolve(JSONObject().put("accel", vector(accel)).put("gyro", vector(gyro)))
            "orientation-start" -> {
                val us = 1_000_000 / call.args.optInt("hz", 30).coerceAtLeast(1)
                val rotation = manager.getDefaultSensor(Sensor.TYPE_ROTATION_VECTOR)
                val a = manager.getDefaultSensor(Sensor.TYPE_ACCELEROMETER)
                val m = manager.getDefaultSensor(Sensor.TYPE_MAGNETIC_FIELD)
                when {
                    rotation != null -> manager.registerListener(turned, rotation, us)
                    a != null && m != null -> {
                        manager.registerListener(turned, a, us)
                        manager.registerListener(turned, m, us)
                    }
                    else -> return call.reject(Plugins.UNAVAILABLE, "this phone has no motion sensors")
                }
                call.resolve()
            }
            "orientation-read" -> {
                val v = FloatArray(3)
                SensorManager.getOrientation(matrix, v)
                val degrees = { r: Float -> Math.toDegrees(r.toDouble()) }
                call.resolve(JSONObject().put("pitch", -degrees(v[1])).put("roll", degrees(v[2])).put("yaw", degrees(v[0])))
            }
            "orientation-stop" -> {
                manager.unregisterListener(turned)
                call.resolve()
            }
            "stop" -> {
                manager.unregisterListener(listener)
                call.resolve()
            }
            else -> Plugins.unknownMethod(call)
        }
    }
}
