package uz.kripton.mullajiring.notifier.delivery

import kotlinx.coroutines.flow.Flow
import uz.kripton.mullajiring.notifier.config.AppConfig
import uz.kripton.mullajiring.notifier.data.CardBalanceDao
import uz.kripton.mullajiring.notifier.data.CardBalanceEntity
import uz.kripton.mullajiring.notifier.data.OutboxDao
import uz.kripton.mullajiring.notifier.data.OutboxEntity
import uz.kripton.mullajiring.notifier.data.OutboxStatus
import uz.kripton.mullajiring.notifier.data.ParseFailureDao
import uz.kripton.mullajiring.notifier.data.ParseFailureEntity
import uz.kripton.mullajiring.notifier.net.DeliveryOutcome
import uz.kripton.mullajiring.notifier.net.IngestClient
import uz.kripton.mullajiring.notifier.net.IngestTransactionRequest
import uz.kripton.mullajiring.notifier.parser.IdempotencyKey
import uz.kripton.mullajiring.notifier.parser.ParseResult
import uz.kripton.mullajiring.notifier.parser.SmsParser

/** Signals raised during processing for the caller (service/UI) to surface as notifications. */
sealed interface NotifierEvent {
    data class ParseFailure(val count: Int) : NotifierEvent
    data class PermanentFailure(val externalId: String, val detail: String) : NotifierEvent
    data class ReconciliationGap(val cardLast4: String, val gapMinor: Long, val currency: String) : NotifierEvent
}

class NotifierRepository(
    private val config: AppConfig,
    private val outbox: OutboxDao,
    private val parseFailures: ParseFailureDao,
    private val balances: CardBalanceDao,
    private val ingest: IngestClient,
    private val now: () -> Long = System::currentTimeMillis,
) {

    val pendingCount: Flow<Int> = outbox.pendingCount()
    val failedCount: Flow<Int> = outbox.failedCount()
    val activeParseFailures = parseFailures.active()
    val activeParseFailureCount: Flow<Int> = parseFailures.activeCount()
    val recentOutbox = outbox.recent()

    /**
     * Sender filter + parse + persist. Returns events the caller should surface.
     * OTP / non-transaction messages are dropped silently and never stored.
     */
    suspend fun ingestSms(rawBody: String, sender: String): List<NotifierEvent> {
        if (!sender.trim().equals(config.senderName, ignoreCase = true)) {
            return emptyList() // not our bank — ignore entirely, never log
        }
        val externalId = IdempotencyKey.of(rawBody, sender)
        return when (val result = SmsParser.parse(rawBody)) {
            is ParseResult.Ignored -> emptyList()

            is ParseResult.Failed -> {
                parseFailures.insertIfNew(
                    ParseFailureEntity(externalId, rawBody, sender, result.reason, now()),
                )
                listOf(NotifierEvent.ParseFailure(1))
            }

            is ParseResult.Success -> {
                val tx = result.transaction
                outbox.insertIfNew(
                    OutboxEntity(
                        externalId = externalId,
                        type = tx.type.wire,
                        merchant = tx.merchant,
                        cardLast4 = tx.cardLast4,
                        amountMinor = tx.amountMinor,
                        currency = tx.currency,
                        balanceMinor = tx.balanceMinor,
                        balanceCurrency = tx.balanceCurrency,
                        occurredAtUtc = tx.occurredAtUtc,
                        status = OutboxStatus.PENDING,
                        attempts = 0,
                        nextAttemptAt = now(),
                        createdAt = now(),
                    ),
                )
                emptyList()
            }
        }
    }

    /** Result of a single drain pass over the outbox. */
    data class DrainResult(
        val events: List<NotifierEvent>,
        val hasPending: Boolean,
        val nextAttemptAt: Long?,
    )

    /** Deliver every due item once; apply backoff/permanent-fail; reconcile on success. */
    suspend fun drainOutbox(): DrainResult {
        val events = mutableListOf<NotifierEvent>()
        for (item in outbox.due(now())) {
            when (val outcome = ingest.deliver(item.toRequest())) {
                is DeliveryOutcome.Success -> {
                    outbox.update(item.externalId, OutboxStatus.DELIVERED, item.attempts, 0, null)
                    reconcile(item)?.let { events += it }
                }

                is DeliveryOutcome.PermanentFailure -> {
                    val detail = "HTTP ${outcome.code}: ${outcome.body.take(200)}"
                    outbox.update(item.externalId, OutboxStatus.FAILED, item.attempts, 0, detail)
                    events += NotifierEvent.PermanentFailure(item.externalId, detail)
                }

                is DeliveryOutcome.Retryable -> {
                    if (BackoffPolicy.isExpired(item.createdAt, now())) {
                        val detail = "expired after 24h: ${outcome.reason}"
                        outbox.update(item.externalId, OutboxStatus.FAILED, item.attempts, 0, detail)
                        events += NotifierEvent.PermanentFailure(item.externalId, detail)
                    } else {
                        val attempts = item.attempts + 1
                        val next = now() + BackoffPolicy.delayMs(attempts)
                        outbox.update(item.externalId, OutboxStatus.PENDING, attempts, next, outcome.reason)
                    }
                }
            }
        }
        val earliest = outbox.earliestNextAttempt()
        return DrainResult(events, hasPending = earliest != null, nextAttemptAt = earliest)
    }

    /**
     * Compare the new balance against (last balance − amount). A gap beyond the
     * per-currency threshold means a message was missed. Only meaningful when the
     * amount and balance share a currency (the card's own currency).
     */
    private suspend fun reconcile(item: OutboxEntity): NotifierEvent? {
        val prior = balances.get(item.cardLast4)
        var gapEvent: NotifierEvent? = null
        if (prior != null &&
            prior.currency == item.balanceCurrency &&
            item.currency == item.balanceCurrency
        ) {
            val expected = prior.balanceMinor - item.amountMinor // expense reduces balance
            val gap = kotlin.math.abs(item.balanceMinor - expected)
            if (gap > config.reconThresholdMinor(item.balanceCurrency)) {
                gapEvent = NotifierEvent.ReconciliationGap(item.cardLast4, gap, item.balanceCurrency)
            }
        }
        balances.upsert(
            CardBalanceEntity(item.cardLast4, item.balanceMinor, item.balanceCurrency, now()),
        )
        return gapEvent
    }

    suspend fun dismissParseFailure(externalId: String) = parseFailures.dismiss(externalId)

    private fun OutboxEntity.toRequest() = IngestTransactionRequest(
        externalId = externalId,
        type = type,
        amount = amountMinor,
        fromCardLast4 = cardLast4,
        merchant = merchant,
        date = occurredAtUtc,
    )
}
