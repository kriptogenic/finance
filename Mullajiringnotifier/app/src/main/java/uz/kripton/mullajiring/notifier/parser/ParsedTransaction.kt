package uz.kripton.mullajiring.notifier.parser

/**
 * A successfully parsed bank transaction. All money is integer minor units.
 */
data class ParsedTransaction(
    val type: TransactionType,
    val merchant: String,
    val maskedCard: String,
    val cardLast4: String,
    val amountMinor: Long,
    val currency: String,
    val balanceMinor: Long,
    val balanceCurrency: String,
    /** Transaction time as ISO-8601 UTC (e.g. 2026-06-18T05:08:41Z). */
    val occurredAtUtc: String,
)

enum class TransactionType(val wire: String) {
    EXPENSE("expense"),
    INCOME("income"),
    TRANSFER("transfer"),
}

/** Outcome of parsing a single SMS body. */
sealed interface ParseResult {
    data class Success(val transaction: ParsedTransaction) : ParseResult

    /** Sender is infinbank but the message is not a transaction (OTP, info). Drop silently. */
    data object Ignored : ParseResult

    /** Looks like a transaction (Pokupka prefix) but fields could not be parsed. Log + notify. */
    data class Failed(val reason: String) : ParseResult
}
