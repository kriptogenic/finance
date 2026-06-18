package uz.kripton.mullajiring.notifier.sms

import android.content.Context
import android.provider.Telephony
import uz.kripton.mullajiring.notifier.Graph
import uz.kripton.mullajiring.notifier.delivery.DeliveryWorker
import uz.kripton.mullajiring.notifier.parser.SmsParser

/**
 * First-launch inbox backfill: read existing infinbank SMS and process any
 * undelivered transactions. Idempotent by the shared key scheme, so safe to repeat.
 */
object SmsBackfill {

    suspend fun run(context: Context) {
        val config = Graph.config(context)
        if (config.firstLaunchBackfillDone) return

        val repo = Graph.repository(context)
        val resolver = context.contentResolver
        val cursor = resolver.query(
            Telephony.Sms.Inbox.CONTENT_URI,
            arrayOf(Telephony.Sms.ADDRESS, Telephony.Sms.BODY),
            "${Telephony.Sms.ADDRESS} LIKE ?",
            arrayOf(SmsParser.SENDER),
            "${Telephony.Sms.DATE} ASC",
        ) ?: return

        cursor.use { c ->
            val addressIdx = c.getColumnIndexOrThrow(Telephony.Sms.ADDRESS)
            val bodyIdx = c.getColumnIndexOrThrow(Telephony.Sms.BODY)
            while (c.moveToNext()) {
                val sender = c.getString(addressIdx) ?: continue
                val body = c.getString(bodyIdx) ?: continue
                repo.ingestSms(body, sender) // events suppressed during bulk backfill
            }
        }
        config.firstLaunchBackfillDone = true
        DeliveryWorker.schedule(context)
    }
}
