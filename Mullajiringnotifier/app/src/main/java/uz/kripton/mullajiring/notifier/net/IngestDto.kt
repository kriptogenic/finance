package uz.kripton.mullajiring.notifier.net

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * Mirror of the finance app's IngestTransactionRequest (specs/api.yaml).
 * The caller sends card + merchant; the finance app resolves account & category.
 */
@Serializable
data class IngestTransactionRequest(
    @SerialName("external_id") val externalId: String,
    val type: String,
    val amount: Long,
    @SerialName("from_card_last4") val fromCardLast4: String? = null,
    @SerialName("to_card_last4") val toCardLast4: String? = null,
    val merchant: String? = null,
    val date: String? = null,
    @SerialName("to_amount") val toAmount: Long? = null,
    @SerialName("rate_to_base") val rateToBase: String? = null,
    val tags: List<String>? = null,
)

/**
 * Mirror of the finance app's BalanceSnapshotRequest (specs/api.yaml).
 * Upserts the latest reported balance per card for server-side reconciliation.
 */
@Serializable
data class BalanceSnapshotRequest(
    @SerialName("reported_at") val reportedAt: String,
    val source: String? = null,
    val balances: List<BalanceSnapshotEntry>,
)

@Serializable
data class BalanceSnapshotEntry(
    @SerialName("card_last4") val cardLast4: String,
    val bank: String? = null,
    val amount: Long,
    val currency: String,
)

/** Result of one delivery attempt. */
sealed interface DeliveryOutcome {
    /** 2xx — created (201) or already ingested (200). Treat both as done. */
    data object Success : DeliveryOutcome

    /** 4xx other than auth — bad/contract error. Do not retry; alert the user. */
    data class PermanentFailure(val code: Int, val body: String) : DeliveryOutcome

    /** Network error or 5xx — enqueue for backoff retry. */
    data class Retryable(val reason: String) : DeliveryOutcome
}
