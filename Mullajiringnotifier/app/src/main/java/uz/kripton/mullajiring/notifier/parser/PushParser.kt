package uz.kripton.mullajiring.notifier.parser

/**
 * Pure parser for Ipak Yuli mobile push notifications. No Android dependencies — unit-tested on the JVM.
 *
 * The listener hands us the notification title + body joined with newlines, e.g.:
 *   Оплата по карте Visa *2217
 *   Сумма операции: 21 000 UZS
 *   Доступно: 101 979 UZS
 *   Магазин: ООО SD MART
 *   Место: Toshkent, UZ
 *   Дата: 2026-06-26 11:28:49
 *
 * Only card-payment pushes are processed. Anything else (OTP, login, top-up) is dropped silently
 * and never logged — same security stance as OTP SMS.
 */
object PushParser {

    const val PACKAGE = "com.ipakyulibank.mobile"

    // Only this phrase marks a card purchase; its absence => not a transaction => drop silently.
    private const val PURCHASE_MARKER = "Оплата по карте"

    private val CARD = Regex("""\*\s?(\d{4})""")
    private val AMOUNT = Regex("""Сумма операции:\s*([\d ]+(?:[.,]\d{1,2})?)\s*([A-Z]{3})""")
    private val BALANCE = Regex("""Доступно:\s*([\d ]+(?:[.,]\d{1,2})?)\s*([A-Z]{3})""")
    private val MERCHANT = Regex("""Магазин:[ \t]*(.+?)[ \t]*(?:\R|Место:|Дата:|$)""")
    private val DATE = Regex("""Дата:\s*(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})""")

    fun parse(rawText: String): ParseResult {
        val body = normalize(rawText)
        if (!body.contains(PURCHASE_MARKER)) {
            // OTP / login / informational push — drop silently, never log.
            return ParseResult.Ignored
        }

        val card = CARD.find(body)?.groupValues?.get(1)
            ?: return ParseResult.Failed("Push: card number not found")
        val amountMatch = AMOUNT.find(body)
            ?: return ParseResult.Failed("Push: amount not found")
        val merchant = MERCHANT.find(body)?.groupValues?.get(1)?.trim()?.takeIf { it.isNotEmpty() }
            ?: return ParseResult.Failed("Push: merchant not found")
        val dateRaw = DATE.find(body)?.groupValues?.get(1)
            ?: return ParseResult.Failed("Push: date not found")

        val amount = SmsParser.parseMinor(amountMatch.groupValues[1])
            ?: return ParseResult.Failed("Push: unparseable amount '${amountMatch.groupValues[1]}'")
        val occurredAt = BankTime.toUtcIso(dateRaw)
            ?: return ParseResult.Failed("Push: unparseable date '$dateRaw'")

        // Balance ("Доступно") is reconciliation-only; fall back to the amount currency if absent.
        val balanceMatch = BALANCE.find(body)
        val balance = balanceMatch?.let { SmsParser.parseMinor(it.groupValues[1]) } ?: 0L
        val balanceCurrency = balanceMatch?.groupValues?.get(2) ?: amountMatch.groupValues[2]

        return ParseResult.Success(
            ParsedTransaction(
                type = TransactionType.EXPENSE,
                merchant = merchant, // keep original casing/spacing
                maskedCard = "*$card",
                cardLast4 = card,
                amountMinor = amount,
                currency = amountMatch.groupValues[2],
                balanceMinor = balance,
                balanceCurrency = balanceCurrency,
                occurredAtUtc = occurredAt,
            ),
        )
    }

    // No-break (U+00A0), thin (U+2009) and narrow no-break (U+202F) spaces used as group separators.
    private val UNICODE_SPACES = charArrayOf('\u00A0', '\u2009', '\u202F')

    /** Collapse the various unicode thin/no-break spaces banks use into plain ASCII spaces. */
    private fun normalize(text: String): String {
        var s = text
        for (c in UNICODE_SPACES) s = s.replace(c, ' ')
        return s.trim()
    }
}
