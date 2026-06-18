package uz.kripton.mullajiring.notifier.delivery

/** Exponential backoff for the retry queue: 30s initial, capped at 30 min; give up after 24h. */
object BackoffPolicy {
    const val INITIAL_MS = 30_000L
    const val MAX_MS = 30 * 60_000L
    const val MAX_AGE_MS = 24 * 60 * 60_000L

    /** Delay before the n-th attempt (attempts already made = [priorAttempts]). */
    fun delayMs(priorAttempts: Int): Long {
        if (priorAttempts <= 0) return INITIAL_MS
        val shift = priorAttempts.coerceAtMost(20)
        val scaled = INITIAL_MS shl shift
        return if (scaled <= 0 || scaled > MAX_MS) MAX_MS else scaled
    }

    fun isExpired(createdAt: Long, now: Long): Boolean = now - createdAt > MAX_AGE_MS
}
