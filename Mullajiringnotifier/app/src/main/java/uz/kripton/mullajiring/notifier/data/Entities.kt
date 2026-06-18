package uz.kripton.mullajiring.notifier.data

import androidx.room.Entity
import androidx.room.PrimaryKey

enum class OutboxStatus { PENDING, DELIVERED, FAILED }

/**
 * One parsed transaction awaiting (or completed) delivery. Keyed by the SHA-256
 * idempotency key so OS redelivery and inbox backfill collapse onto one row.
 */
@Entity(tableName = "outbox")
data class OutboxEntity(
    @PrimaryKey val externalId: String,
    val type: String,
    val merchant: String,
    val cardLast4: String,
    val amountMinor: Long,
    val currency: String,
    val balanceMinor: Long,
    val balanceCurrency: String,
    val occurredAtUtc: String,
    val status: OutboxStatus,
    val attempts: Int,
    val nextAttemptAt: Long,
    val createdAt: Long,
    val lastError: String? = null,
)

/** An infinbank message that looked like a transaction but failed to parse. Never silently dropped. */
@Entity(tableName = "parse_failures")
data class ParseFailureEntity(
    @PrimaryKey val externalId: String,
    val rawBody: String,
    val sender: String,
    val reason: String,
    val receivedAt: Long,
    val dismissed: Boolean = false,
)

/** Last known balance per card, used for missed-message reconciliation. */
@Entity(tableName = "card_balances")
data class CardBalanceEntity(
    @PrimaryKey val cardLast4: String,
    val balanceMinor: Long,
    val currency: String,
    val updatedAt: Long,
)
