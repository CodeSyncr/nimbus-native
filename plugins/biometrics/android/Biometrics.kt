package space.nimbusgo.nativekit

import android.content.Context
import android.hardware.biometrics.BiometricManager
import android.hardware.biometrics.BiometricPrompt
import android.os.Build
import android.os.CancellationSignal

/** biometrics: the platform's prompt, which needs no support library (Android 10+). */
internal object BiometricsPlugin {
    private fun ready(): Boolean {
        if (Build.VERSION.SDK_INT < 30) return false
        val m = Plugins.context.getSystemService(Context.BIOMETRIC_SERVICE) as BiometricManager
        return m.canAuthenticate(BiometricManager.Authenticators.BIOMETRIC_STRONG) == BiometricManager.BIOMETRIC_SUCCESS
    }

    fun handle(call: PluginCall) {
        when (call.method) {
            "available" -> call.resolve(mapOf("kind" to if (ready()) "fingerprint" else ""))
            "confirm" -> {
                if (!ready()) return call.reject(Plugins.UNAVAILABLE, "no biometrics are set up on this phone")
                val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no screen to open it from")
                val prompt = BiometricPrompt.Builder(a).setTitle(call.string("reason") ?: "Confirm it's you")
                    .setNegativeButton("Cancel", a.mainExecutor) { _, _ -> call.resolve(mapOf("ok" to false)) }
                    .build()
                prompt.authenticate(CancellationSignal(), a.mainExecutor, object : BiometricPrompt.AuthenticationCallback() {
                    override fun onAuthenticationSucceeded(r: BiometricPrompt.AuthenticationResult) = call.resolve(mapOf("ok" to true))
                    override fun onAuthenticationError(code: Int, s: CharSequence) = call.resolve(mapOf("ok" to false))
                })
            }
            else -> Plugins.unknownMethod(call)
        }
    }
}
