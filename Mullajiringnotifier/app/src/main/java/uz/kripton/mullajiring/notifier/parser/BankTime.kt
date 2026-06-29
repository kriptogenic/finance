package uz.kripton.mullajiring.notifier.parser

import java.time.LocalDateTime
import java.time.ZoneId
import java.time.format.DateTimeFormatter

/** Bank timestamps are Asia/Tashkent local; we store and transmit them as ISO-8601 UTC. */
object BankTime {

    private val TASHKENT: ZoneId = ZoneId.of("Asia/Tashkent")
    private val LOCAL: DateTimeFormatter = DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm:ss")

    /** `2026-06-26 11:28:49` (Tashkent) -> `2026-06-26T06:28:49Z`, or null if unparseable. */
    fun toUtcIso(datetime: String): String? = try {
        val local = LocalDateTime.parse(datetime.trim(), LOCAL)
        DateTimeFormatter.ISO_INSTANT.format(local.atZone(TASHKENT).toInstant())
    } catch (_: Exception) {
        null
    }
}
