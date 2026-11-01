package space.nimbusgo.nativekit

import android.os.CancellationSignal
import androidx.credentials.CredentialManager
import androidx.credentials.CredentialManagerCallback
import androidx.credentials.CustomCredential
import androidx.credentials.GetCredentialRequest
import androidx.credentials.GetCredentialResponse
import androidx.credentials.exceptions.GetCredentialCancellationException
import androidx.credentials.exceptions.GetCredentialException
import androidx.credentials.exceptions.NoCredentialException
import com.google.android.libraries.identity.googleid.GetGoogleIdOption
import com.google.android.libraries.identity.googleid.GoogleIdTokenCredential

/** signin: Google through the phone's Credential Manager. Apple has no Android sheet. */
internal object SignInPlugin {
    fun handle(call: PluginCall) {
        when (call.method) {
            "apple" -> call.reject(Plugins.UNAVAILABLE, "Sign in with Apple is for iOS")
            "google" -> google(call)
            else -> Plugins.unknownMethod(call)
        }
    }

    private fun google(call: PluginCall) {
        val client = call.string("androidClient")
        if (client.isNullOrEmpty()) return call.reject(Plugins.FAILED, "Google sign-in needs AndroidClient")
        val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no screen to open it from")
        val option = GetGoogleIdOption.Builder().setServerClientId(client).setFilterByAuthorizedAccounts(false)
        call.string("nonce")?.let { option.setNonce(it) }
        val request = GetCredentialRequest.Builder().addCredentialOption(option.build()).build()
        CredentialManager.create(a).getCredentialAsync(a, request, CancellationSignal(), a.mainExecutor,
            object : CredentialManagerCallback<GetCredentialResponse, GetCredentialException> {
                override fun onResult(r: GetCredentialResponse) {
                    val c = r.credential
                    if (c !is CustomCredential || c.type != GoogleIdTokenCredential.TYPE_GOOGLE_ID_TOKEN_CREDENTIAL) {
                        return call.reject(Plugins.FAILED, "the phone sent another kind of sign-in")
                    }
                    val g = GoogleIdTokenCredential.createFrom(c.data)
                    call.resolve(mapOf("idToken" to g.idToken, "name" to (g.displayName ?: "")))
                }

                override fun onError(e: GetCredentialException) = when (e) {
                    is GetCredentialCancellationException -> call.reject(Plugins.CANCELLED, "they backed out")
                    is NoCredentialException -> call.reject(Plugins.UNAVAILABLE, "no Google account on this phone")
                    else -> call.reject(Plugins.FAILED, e.message ?: "Google sign-in failed")
                }
            })
    }
}
