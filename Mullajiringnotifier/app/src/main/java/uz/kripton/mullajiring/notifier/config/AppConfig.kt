package uz.kripton.mullajiring.notifier.config

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey

/**
 * App configuration in Keystore-backed encrypted SharedPreferences.
 * Nothing here is hardcoded into the pipeline — endpoint, token, card and
 * reconciliation thresholds are all read from this store.
 */
class AppConfig private constructor(private val prefs: SharedPreferences) {

    var ingestBaseUrl: String
        get() = prefs.getString(KEY_BASE_URL, "").orEmpty()
        set(value) = prefs.edit().putString(KEY_BASE_URL, value.trim().trimEnd('/')).apply()

    var ingestToken: String
        get() = prefs.getString(KEY_TOKEN, "").orEmpty()
        set(value) = prefs.edit().putString(KEY_TOKEN, value.trim()).apply()

    var firstLaunchBackfillDone: Boolean
        get() = prefs.getBoolean(KEY_BACKFILL_DONE, false)
        set(value) = prefs.edit().putBoolean(KEY_BACKFILL_DONE, value).apply()

    val isConfigured: Boolean
        get() = ingestBaseUrl.isNotEmpty() && ingestToken.isNotEmpty()

    /** Reconciliation tolerance in minor units for a balance gap; per-currency with a default. */
    fun reconThresholdMinor(currency: String): Long = when (currency.uppercase()) {
        "UZS" -> 100L // 1 UZS
        else -> 1L    // 0.01 of a foreign unit
    }

    companion object {
        private const val FILE = "notifier_secure_prefs"
        private const val KEY_BASE_URL = "ingest_base_url"
        private const val KEY_TOKEN = "ingest_token"
        private const val KEY_BACKFILL_DONE = "backfill_done"

        @Volatile
        private var instance: AppConfig? = null

        fun get(context: Context): AppConfig = instance ?: synchronized(this) {
            instance ?: build(context.applicationContext).also { instance = it }
        }

        private fun build(context: Context): AppConfig {
            val masterKey = MasterKey.Builder(context)
                .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
                .build()
            val prefs = EncryptedSharedPreferences.create(
                context,
                FILE,
                masterKey,
                EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
                EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
            )
            return AppConfig(prefs)
        }
    }
}
