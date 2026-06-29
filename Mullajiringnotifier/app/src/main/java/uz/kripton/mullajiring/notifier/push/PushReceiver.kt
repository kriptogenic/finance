package uz.kripton.mullajiring.notifier.push

import android.app.Notification
import android.service.notification.NotificationListenerService
import android.service.notification.StatusBarNotification
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch
import uz.kripton.mullajiring.notifier.Graph
import uz.kripton.mullajiring.notifier.delivery.DeliveryWorker
import uz.kripton.mullajiring.notifier.notify.Notifications

/**
 * Real-time bank-app push notifications. Mirrors [uz.kripton.mullajiring.notifier.sms.SmsReceiver]:
 * filter by source, persist, then kick delivery. Notifications from other apps are never read.
 */
class PushReceiver : NotificationListenerService() {

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)

    override fun onNotificationPosted(sbn: StatusBarNotification) {
        val appContext = applicationContext
        // Source gate first: only touch the configured bank app's notifications.
        if (sbn.packageName != Graph.config(appContext).pushPackage) return

        val raw = extractText(sbn) ?: return
        scope.launch {
            val events = Graph.repository(appContext).ingestPush(raw, sbn.packageName)
            events.forEach { Notifications.notify(appContext, it) }
            DeliveryWorker.schedule(appContext)
        }
    }

    override fun onDestroy() {
        scope.cancel()
        super.onDestroy()
    }

    /** Headline + expanded body joined into the raw text the parser and idempotency key consume. */
    private fun extractText(sbn: StatusBarNotification): String? {
        val extras = sbn.notification.extras ?: return null
        val title = extras.getCharSequence(Notification.EXTRA_TITLE)?.toString().orEmpty()
        val big = extras.getCharSequence(Notification.EXTRA_BIG_TEXT)?.toString()
        val text = extras.getCharSequence(Notification.EXTRA_TEXT)?.toString().orEmpty()
        val raw = listOf(title, big ?: text).filter { it.isNotBlank() }.joinToString("\n")
        return raw.ifBlank { null }
    }
}
