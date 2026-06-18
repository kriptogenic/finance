package uz.kripton.mullajiring.notifier.data

import androidx.room.Dao
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.Query
import androidx.room.Upsert
import kotlinx.coroutines.flow.Flow

@Dao
interface OutboxDao {

    /** Insert only if absent; an existing key (already seen) is kept as-is. */
    @Insert(onConflict = OnConflictStrategy.IGNORE)
    suspend fun insertIfNew(item: OutboxEntity): Long

    @Query("SELECT * FROM outbox WHERE status = 'PENDING' AND nextAttemptAt <= :now ORDER BY createdAt ASC")
    suspend fun due(now: Long): List<OutboxEntity>

    @Query("SELECT MIN(nextAttemptAt) FROM outbox WHERE status = 'PENDING'")
    suspend fun earliestNextAttempt(): Long?

    @Query("SELECT * FROM outbox WHERE externalId = :id")
    suspend fun byId(id: String): OutboxEntity?

    @Query("UPDATE outbox SET status = :status, attempts = :attempts, nextAttemptAt = :next, lastError = :error WHERE externalId = :id")
    suspend fun update(id: String, status: OutboxStatus, attempts: Int, next: Long, error: String?)

    @Query("SELECT COUNT(*) FROM outbox WHERE status = 'PENDING'")
    fun pendingCount(): Flow<Int>

    @Query("SELECT COUNT(*) FROM outbox WHERE status = 'FAILED'")
    fun failedCount(): Flow<Int>

    @Query("SELECT * FROM outbox ORDER BY createdAt DESC LIMIT 100")
    fun recent(): Flow<List<OutboxEntity>>
}

@Dao
interface ParseFailureDao {

    @Insert(onConflict = OnConflictStrategy.IGNORE)
    suspend fun insertIfNew(item: ParseFailureEntity)

    @Query("SELECT * FROM parse_failures WHERE dismissed = 0 ORDER BY receivedAt DESC")
    fun active(): Flow<List<ParseFailureEntity>>

    @Query("SELECT COUNT(*) FROM parse_failures WHERE dismissed = 0")
    fun activeCount(): Flow<Int>

    @Query("UPDATE parse_failures SET dismissed = 1 WHERE externalId = :id")
    suspend fun dismiss(id: String)
}

@Dao
interface CardBalanceDao {

    @Query("SELECT * FROM card_balances WHERE cardLast4 = :card")
    suspend fun get(card: String): CardBalanceEntity?

    @Upsert
    suspend fun upsert(item: CardBalanceEntity)
}
