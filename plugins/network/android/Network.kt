package space.nimbusgo.nativekit

import android.content.Context
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.provider.Settings
import java.net.Inet4Address

/** network: the phone's connection. */
internal object NetworkPlugin {
    fun handle(call: PluginCall) {
        if (call.method != "status") return Plugins.unknownMethod(call)
        val cm = Plugins.context.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
        val caps = cm.activeNetwork?.let { cm.getNetworkCapabilities(it) }
        val on = caps?.hasCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET) == true
        val type = when {
            !on -> "none"
            caps!!.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) -> "wifi"
            caps.hasTransport(NetworkCapabilities.TRANSPORT_CELLULAR) -> "cellular"
            caps.hasTransport(NetworkCapabilities.TRANSPORT_ETHERNET) -> "wired"
            else -> "other"
        }
        val addresses = cm.activeNetwork?.let { cm.getLinkProperties(it) }?.linkAddresses?.map { it.address }.orEmpty()
        val address = (addresses.firstOrNull { it is Inet4Address } ?: addresses.firstOrNull())?.hostAddress ?: ""
        val airplane = Settings.Global.getInt(Plugins.context.contentResolver, Settings.Global.AIRPLANE_MODE_ON, 0) == 1
        call.resolve(mapOf("connected" to on, "type" to type, "expensive" to (on && cm.isActiveNetworkMetered),
            "address" to address, "airplane" to airplane))
    }
}
