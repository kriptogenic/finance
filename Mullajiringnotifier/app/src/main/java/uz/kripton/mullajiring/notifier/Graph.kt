package uz.kripton.mullajiring.notifier

import android.content.Context
import uz.kripton.mullajiring.notifier.config.AppConfig
import uz.kripton.mullajiring.notifier.data.NotifierDatabase
import uz.kripton.mullajiring.notifier.delivery.NotifierRepository
import uz.kripton.mullajiring.notifier.net.IngestClient

/** Tiny manual DI graph — single source of the repository for receivers, workers and UI. */
object Graph {

    @Volatile
    private var repo: NotifierRepository? = null

    @Volatile
    private var cfg: AppConfig? = null

    fun config(context: Context): AppConfig = cfg ?: synchronized(this) {
        cfg ?: AppConfig.get(context).also { cfg = it }
    }

    fun repository(context: Context): NotifierRepository = repo ?: synchronized(this) {
        repo ?: build(context.applicationContext).also { repo = it }
    }

    /** Stateless client for the Settings "Test connection" probe. */
    fun ingestClient(context: Context): IngestClient {
        val config = config(context)
        return IngestClient(
            baseUrlProvider = { config.ingestBaseUrl },
            tokenProvider = { config.ingestToken },
        )
    }

    private fun build(context: Context): NotifierRepository {
        val config = config(context)
        val db = NotifierDatabase.get(context)
        val client = IngestClient(
            baseUrlProvider = { config.ingestBaseUrl },
            tokenProvider = { config.ingestToken },
        )
        return NotifierRepository(
            config = config,
            outbox = db.outboxDao(),
            parseFailures = db.parseFailureDao(),
            balances = db.cardBalanceDao(),
            ingest = client,
        )
    }
}
