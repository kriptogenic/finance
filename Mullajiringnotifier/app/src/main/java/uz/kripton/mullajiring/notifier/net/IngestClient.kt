package uz.kripton.mullajiring.notifier.net

import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import java.io.IOException
import java.util.concurrent.TimeUnit

/**
 * Posts ingest transactions to the finance app. 401 is treated as retryable
 * (token may be re-provisioned); other 4xx are permanent contract failures.
 */
class IngestClient(
    private val baseUrlProvider: () -> String,
    private val tokenProvider: () -> String,
    private val http: OkHttpClient = defaultHttp(),
) {

    fun deliver(request: IngestTransactionRequest): DeliveryOutcome =
        post("/ingest/transactions", json.encodeToString(IngestTransactionRequest.serializer(), request))

    /** Report a card balance snapshot for server-side reconciliation. */
    fun deliverBalance(request: BalanceSnapshotRequest): DeliveryOutcome =
        post("/ingest/balances", json.encodeToString(BalanceSnapshotRequest.serializer(), request))

    /** GET <base>/health to test reachability. Uses the given base, no auth. */
    fun checkHealth(rawBaseUrl: String): String {
        val base = rawBaseUrl.trim().trimEnd('/')
        if (base.isEmpty()) return "Set a base URL first"
        val request = Request.Builder().url("$base/health").get().build()
        return try {
            http.newCall(request).execute().use { resp ->
                if (resp.isSuccessful) "OK ${resp.code}: ${resp.body?.string()?.take(100).orEmpty()}"
                else "HTTP ${resp.code}"
            }
        } catch (e: IllegalArgumentException) {
            "Invalid URL: ${e.message}"
        } catch (e: IOException) {
            "Failed: ${e.message}"
        }
    }

    private fun post(path: String, payload: String): DeliveryOutcome {
        val base = baseUrlProvider()
        val token = tokenProvider()
        if (base.isEmpty() || token.isEmpty()) {
            return DeliveryOutcome.Retryable("not configured")
        }

        val httpRequest = Request.Builder()
            .url("$base$path")
            .header("Authorization", "Bearer $token")
            .post(payload.toRequestBody(JSON))
            .build()

        return try {
            http.newCall(httpRequest).execute().use { resp ->
                when {
                    resp.isSuccessful -> DeliveryOutcome.Success // 2xx (incl. 200 deduped, 204) all ok
                    resp.code == 401 -> DeliveryOutcome.Retryable("401 unauthorized")
                    resp.code in 400..499 ->
                        DeliveryOutcome.PermanentFailure(resp.code, resp.body?.string().orEmpty())
                    else -> DeliveryOutcome.Retryable("server ${resp.code}")
                }
            }
        } catch (e: IOException) {
            DeliveryOutcome.Retryable(e.message ?: "network error")
        }
    }

    companion object {
        private val JSON = "application/json; charset=utf-8".toMediaType()
        private val json = Json { encodeDefaults = false; explicitNulls = false }

        private fun defaultHttp(): OkHttpClient = OkHttpClient.Builder()
            .connectTimeout(15, TimeUnit.SECONDS)
            .readTimeout(20, TimeUnit.SECONDS)
            .build()
    }
}
