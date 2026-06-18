package uz.kripton.mullajiring.notifier.parser

import java.security.MessageDigest

/**
 * Stable idempotency key for an inbound SMS: SHA-256 of the raw body + sender.
 * The same OS-redelivered message yields the same key, so the finance app dedupes it.
 */
object IdempotencyKey {

    fun of(rawBody: String, sender: String): String {
        val digest = MessageDigest.getInstance("SHA-256")
        val bytes = digest.digest("$rawBody|$sender".toByteArray(Charsets.UTF_8))
        return bytes.joinToString("") { "%02x".format(it) }
    }
}
