package space.nimbusgo.nativekit

import com.android.billingclient.api.AcknowledgePurchaseParams
import com.android.billingclient.api.BillingClient
import com.android.billingclient.api.BillingClientStateListener
import com.android.billingclient.api.BillingFlowParams
import com.android.billingclient.api.BillingResult
import com.android.billingclient.api.ConsumeParams
import com.android.billingclient.api.PendingPurchasesParams
import com.android.billingclient.api.ProductDetails
import com.android.billingclient.api.Purchase
import com.android.billingclient.api.PurchasesUpdatedListener
import com.android.billingclient.api.QueryProductDetailsParams
import com.android.billingclient.api.QueryPurchasesParams

/** purchases: Google Play Billing. */
internal object PurchasesPlugin : PurchasesUpdatedListener {
    private val OK = BillingClient.BillingResponseCode.OK
    private var client: BillingClient? = null
    private var buying: PluginCall? = null

    fun handle(call: PluginCall) = connected(call) { c ->
        when (call.method) {
            "products" -> {
                val a = call.args.optJSONArray("ids")
                val ids = List(a?.length() ?: 0) { a!!.getString(it) }
                details(c, ids) { list -> call.resolve(list.map(::describe)) }
            }
            "buy" -> buy(c, call)
            "finish" -> finish(c, call)
            "consume" -> {
                val p = ConsumeParams.newBuilder().setPurchaseToken(call.string("id") ?: "").build()
                c.consumeAsync(p) { r, _ -> answer(call, r) }
            }
            "owned", "restore" -> owned(c) { list ->
                call.resolve(list.filter { it.purchaseState == Purchase.PurchaseState.PURCHASED }.map(::receipt))
            }
            else -> Plugins.unknownMethod(call)
        }
    }

    private fun connected(call: PluginCall, then: (BillingClient) -> Unit) {
        client?.takeIf { it.isReady }?.let { return then(it) }
        val c = BillingClient.newBuilder(Plugins.context).setListener(this)
            .enablePendingPurchases(PendingPurchasesParams.newBuilder().enableOneTimeProducts().build())
            .build()
        client = c
        c.startConnection(object : BillingClientStateListener {
            override fun onBillingSetupFinished(r: BillingResult) {
                if (r.responseCode == OK) then(c)
                else call.reject(Plugins.UNAVAILABLE, "Google Play can't sell things here: ${r.debugMessage}")
            }
            override fun onBillingServiceDisconnected() {} // the next call connects again
        })
    }

    /** Asks about one-time products and subscriptions: a query can't mix them. */
    private fun details(c: BillingClient, ids: List<String>, done: (List<ProductDetails>) -> Unit) {
        val found = mutableListOf<ProductDetails>()
        fun ask(type: String, next: () -> Unit) {
            if (ids.isEmpty()) return next()
            val q = ids.map { QueryProductDetailsParams.Product.newBuilder().setProductId(it).setProductType(type).build() }
            c.queryProductDetailsAsync(QueryProductDetailsParams.newBuilder().setProductList(q).build()) { _, list ->
                found += list
                next()
            }
        }
        ask(BillingClient.ProductType.INAPP) { ask(BillingClient.ProductType.SUBS) { done(found) } }
    }

    private fun owned(c: BillingClient, done: (List<Purchase>) -> Unit) {
        val params = { type: String -> QueryPurchasesParams.newBuilder().setProductType(type).build() }
        c.queryPurchasesAsync(params(BillingClient.ProductType.INAPP)) { _, one ->
            c.queryPurchasesAsync(params(BillingClient.ProductType.SUBS)) { _, subs -> done(one + subs) }
        }
    }

    private fun buy(c: BillingClient, call: PluginCall) {
        val id = call.string("id") ?: ""
        if (buying != null) return call.reject(Plugins.FAILED, "a purchase is already open")
        details(c, listOf(id)) { list ->
            val d = list.firstOrNull() ?: return@details call.reject(Plugins.NOT_FOUND, "Google Play has no such product")
            val a = Plugins.activity ?: return@details call.reject(Plugins.FAILED, "no screen to open it from")
            val item = BillingFlowParams.ProductDetailsParams.newBuilder().setProductDetails(d)
            baseOffer(d)?.let { item.setOfferToken(it.offerToken) }
            val params = BillingFlowParams.newBuilder().setProductDetailsParamsList(listOf(item.build())).build()
            a.runOnUiThread {
                buying = call
                val r = c.launchBillingFlow(a, params)
                if (r.responseCode != OK) {
                    buying = null
                    call.reject(Plugins.FAILED, r.debugMessage)
                }
            }
        }
    }

    override fun onPurchasesUpdated(r: BillingResult, purchases: MutableList<Purchase>?) {
        val call = buying ?: return
        buying = null
        val p = purchases?.firstOrNull()
        when {
            r.responseCode == BillingClient.BillingResponseCode.USER_CANCELED -> call.reject(Plugins.CANCELLED, "they backed out")
            r.responseCode != OK || p == null -> call.reject(Plugins.FAILED, r.debugMessage)
            p.purchaseState == Purchase.PurchaseState.PENDING -> call.resolve(mapOf("pending" to true))
            else -> call.resolve(receipt(p))
        }
    }

    /** A server may have acknowledged it already, and Play refuses a second one. */
    private fun finish(c: BillingClient, call: PluginCall) = owned(c) { list ->
        val p = list.firstOrNull { it.purchaseToken == call.string("id") }
        if (p == null || p.isAcknowledged) return@owned call.resolve()
        val params = AcknowledgePurchaseParams.newBuilder().setPurchaseToken(p.purchaseToken).build()
        c.acknowledgePurchase(params) { r -> answer(call, r) }
    }

    private fun answer(call: PluginCall, r: BillingResult) {
        if (r.responseCode == OK) call.resolve() else call.reject(Plugins.FAILED, r.debugMessage)
    }

    private fun baseOffer(d: ProductDetails) =
        d.subscriptionOfferDetails?.let { list -> list.firstOrNull { it.offerId == null } ?: list.firstOrNull() }

    private fun receipt(p: Purchase) = mapOf(
        "platform" to "google", "productId" to (p.products.firstOrNull() ?: ""), "id" to p.purchaseToken,
        "token" to p.purchaseToken, "finished" to p.isAcknowledged,
    )

    private fun describe(d: ProductDetails): Map<String, Any?> {
        val once = d.oneTimePurchaseOfferDetails
        val phase = baseOffer(d)?.pricingPhases?.pricingPhaseList?.lastOrNull()
        return mapOf(
            "id" to d.productId, "title" to d.name, "description" to d.description,
            "price" to (once?.formattedPrice ?: phase?.formattedPrice ?: ""),
            "amount" to (once?.priceAmountMicros ?: phase?.priceAmountMicros ?: 0L) / 1_000_000.0,
            "currency" to (once?.priceCurrencyCode ?: phase?.priceCurrencyCode ?: ""),
            "period" to (phase?.billingPeriod ?: ""),
        )
    }
}
