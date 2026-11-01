package space.nimbusgo.nativekit

import android.content.Context
import android.telephony.TelephonyManager

/** cellular: what the telephony service says with no permission. */
internal object CellularPlugin {
    fun handle(call: PluginCall) {
        if (call.method != "get") return Plugins.unknownMethod(call)
        val t = Plugins.context.getSystemService(Context.TELEPHONY_SERVICE) as TelephonyManager
        val code = t.simOperator ?: ""
        call.resolve(mapOf(
            "hasSim" to (t.simState == TelephonyManager.SIM_STATE_READY),
            "carrier" to (t.networkOperatorName ?: ""),
            "country" to (t.networkCountryIso ?: ""),
            "mcc" to code.take(3), "mnc" to code.drop(3),
            "generation" to "", "roaming" to t.isNetworkRoaming,
        ))
    }
}
