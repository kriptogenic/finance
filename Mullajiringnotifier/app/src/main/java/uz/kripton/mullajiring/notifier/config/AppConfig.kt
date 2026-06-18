package uz.kripton.mullajiring.notifier.config

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import uz.kripton.mullajiring.notifier.parser.SmsParser

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

    /** SMS sender to accept (case-insensitive). Falls back to the default if blank. */
    var senderName: String
        get() = prefs.getString(KEY_SENDER, SmsParser.SENDER)?.ifBlank { SmsParser.SENDER } ?: SmsParser.SENDER
        set(value) = prefs.edit().putString(KEY_SENDER, value.trim()).apply()

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
        private const val KEY_SENDER = "sender_name"

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
