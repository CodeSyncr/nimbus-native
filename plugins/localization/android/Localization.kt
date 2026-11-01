package space.nimbusgo.nativekit

import android.os.LocaleList
import android.text.TextUtils
import android.text.format.DateFormat
import android.view.View
import java.text.DecimalFormatSymbols
import java.util.Calendar
import java.util.Currency
import java.util.Locale
import java.util.TimeZone

/** localization: the default locale. Android has no metric setting, so it goes by region. */
internal object LocalizationPlugin {
    private val imperial = setOf("US", "LR", "MM")

    fun handle(call: PluginCall) {
        if (call.method != "get") return Plugins.unknownMethod(call)
        val l = Locale.getDefault()
        val list = LocaleList.getDefault()
        val money = try { Currency.getInstance(l) } catch (e: IllegalArgumentException) { null }
        val digits = DecimalFormatSymbols.getInstance(l)
        call.resolve(mapOf(
            "languages" to List(list.size()) { list[it].toLanguageTag() }, "language" to l.language, "region" to l.country,
            "currency" to (money?.currencyCode ?: ""), "currencySymbol" to (money?.getSymbol(l) ?: ""),
            "timeZone" to TimeZone.getDefault().id, "uses24Hour" to DateFormat.is24HourFormat(Plugins.context),
            "firstWeekday" to Calendar.getInstance(l).firstDayOfWeek, "metric" to (l.country !in imperial),
            "decimalSeparator" to digits.decimalSeparator.toString(), "groupingSeparator" to digits.groupingSeparator.toString(),
            "rightToLeft" to (TextUtils.getLayoutDirectionFromLocale(l) == View.LAYOUT_DIRECTION_RTL),
        ))
    }
}
