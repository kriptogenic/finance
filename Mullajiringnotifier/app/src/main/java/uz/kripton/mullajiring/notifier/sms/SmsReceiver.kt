package uz.kripton.mullajiring.notifier.sms

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.provider.Telephony
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import uz.kripton.mullajiring.notifier.Graph
import uz.kripton.mullajiring.notifier.delivery.DeliveryWorker
import uz.kripton.mullajiring.notifier.notify.Notifications

/** Real-time inbound SMS. Reassembles multipart messages, persists, then kicks delivery. */
class SmsReceiver : BroadcastReceiver() {

    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != Telephony.Sms.Intents.SMS_RECEIVED_ACTION) return

        // Reassemble multipart parts by originating address.
        val bySender = LinkedHashMap<String, StringBuilder>()
        for (msg in Telephony.Sms.Intents.getMessagesFromIntent(intent) ?: return) {
            val sender = msg.originatingAddress ?: continue
            bySender.getOrPut(sender) { StringBuilder() }.append(msg.messageBody ?: "")
        }
        if (bySender.isEmpty()) return

        val appContext = context.applicationContext
        val pending = goAsync()
        CoroutineScope(Dispatchers.IO).launch {
            try {
                val repo = Graph.repository(appContext)
                for ((sender, body) in bySender) {
                    val events = repo.ingestSms(body.toString(), sender)
                    events.forEach { Notifications.notify(appContext, it) }
                }
                // Cheap no-op when nothing is due; covers the common single-transaction case.
                DeliveryWorker.schedule(appContext)
            } finally {
                pending.finish()
            }
        }
    }
}
