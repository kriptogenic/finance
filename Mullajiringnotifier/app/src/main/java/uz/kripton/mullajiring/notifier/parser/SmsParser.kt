package uz.kripton.mullajiring.notifier.parser

/**
 * Pure parser for Infinbank SMS notifications. No Android dependencies — unit-tested on the JVM.
 *
 * Template:
 *   Pokupka: {merchant} {city} {country} {datetime}, karta {masked_card}. summa: {amount} {currency}, balans: {balance} {currency}
 */
object SmsParser {

    const val SENDER = "infinbank"


    //   (NBSP) is interpreted by java.util.regex even inside a raw string.
    // merchant is lazy; the unique datetime anchors the country (the [A-Z]{3} just before it).
    private val PURCHASE = Regex(
        """^Pokupka:\s+(.+?)\s+([A-Z]{3})\s+(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}),\s*karta\s+(\d{6}\*{3}\d{4})\.\s*summa:\s*([\d  ]+[.,]\d{2})\s*([A-Z]{3}),\s*balans:\s*([\d  ]+[.,]\d{2})\s*([A-Z]{3})\s*$""",
    )

    /** True only for transaction messages we attempt to fully parse. */
    private fun isPurchase(body: String): Boolean = body.trimStart().startsWith("Pokupka:")

    fun parse(rawBody: String): ParseResult {
        val body = rawBody.trim()
        if (!isPurchase(body)) {
            // OTP / informational message from infinbank — drop silently, never log.
            return ParseResult.Ignored
        }

        val m = PURCHASE.matchEntire(body)
            ?: return ParseResult.Failed("Pokupka message did not match expected template")

        val (merchantRaw, _, datetime, card, amountRaw, currency, balanceRaw, balanceCurrency) =
            m.destructured

        val amount = parseMinor(amountRaw)
            ?: return ParseResult.Failed("Unparseable amount: '$amountRaw'")
        val balance = parseMinor(balanceRaw)
            ?: return ParseResult.Failed("Unparseable balance: '$balanceRaw'")
        val occurredAt = parseTimestamp(datetime)
            ?: return ParseResult.Failed("Unparseable datetime: '$datetime'")

        return ParseResult.Success(
            ParsedTransaction(
                type = TransactionType.EXPENSE,
                merchant = merchantRaw.trim(), // keep original casing/spacing
                maskedCard = card,
                cardLast4 = card.takeLast(4),
                amountMinor = amount,
                currency = currency,
                balanceMinor = balance,
                balanceCurrency = balanceCurrency,
                occurredAtUtc = occurredAt,
            ),
        )
    }

    /**
     * European-formatted amount -> integer minor units, never via floating point.
     * Strips space / NBSP thousands separators, treats comma or dot as the decimal point.
     */
    fun parseMinor(raw: String): Long? {
        val cleaned = raw.replace(" ", "").replace(" ", "").replace(',', '.')
        val dot = cleaned.indexOf('.')
        return try {
            if (dot < 0) {
                cleaned.toLong() * 100
            } else {
                val intPart = cleaned.substring(0, dot)
                val fracPart = cleaned.substring(dot + 1).padEnd(2, '0').take(2)
                val whole = if (intPart.isEmpty()) 0L else intPart.toLong()
                whole * 100 + fracPart.toLong()
            }
        } catch (_: NumberFormatException) {
            null
        }
    }

    private fun parseTimestamp(datetime: String): String? = BankTime.toUtcIso(datetime)
}
