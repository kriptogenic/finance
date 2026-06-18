package uz.kripton.mullajiring.notifier.notify

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Context
import androidx.core.app.NotificationManagerCompat
import uz.kripton.mullajiring.notifier.R
import uz.kripton.mullajiring.notifier.delivery.NotifierEvent

object Notifications {

    const val CHANNEL_SERVICE = "delivery_service"
    const val CHANNEL_ALERTS = "alerts"

    const val SERVICE_NOTIFICATION_ID = 1
    private var alertId = 1000

    fun ensureChannels(context: Context) {
        val nm = context.getSystemService(NotificationManager::class.java)
        nm.createNotificationChannel(
            NotificationChannel(
                CHANNEL_SERVICE,
                context.getString(R.string.channel_service),
                NotificationManager.IMPORTANCE_LOW,
            ),
        )
        nm.createNotificationChannel(
            NotificationChannel(
                CHANNEL_ALERTS,
                context.getString(R.string.channel_alerts),
                NotificationManager.IMPORTANCE_DEFAULT,
            ),
        )
    }

    /** Ongoing notification that backs the foreground delivery worker. */
    fun serviceNotification(context: Context): Notification =
        Notification.Builder(context, CHANNEL_SERVICE)
            .setContentTitle(context.getString(R.string.service_title))
            .setContentText(context.getString(R.string.service_text))
            .setSmallIcon(android.R.drawable.stat_sys_upload)
            .setOngoing(true)
            .build()

    fun notify(context: Context, event: NotifierEvent) {
        val (title, text) = when (event) {
            is NotifierEvent.ParseFailure ->
                context.getString(R.string.alert_parse_title) to
                    context.getString(R.string.alert_parse_text)

            is NotifierEvent.PermanentFailure ->
                context.getString(R.string.alert_delivery_title) to event.detail

            is NotifierEvent.ReconciliationGap ->
                context.getString(R.string.alert_recon_title) to
                    context.getString(R.string.alert_recon_text, event.cardLast4)
        }
        post(context, title, text)
    }

    private fun post(context: Context, title: String, text: String) {
        if (NotificationManagerCompat.from(context).areNotificationsEnabled()) {
            val n = Notification.Builder(context, CHANNEL_ALERTS)
                .setContentTitle(title)
                .setContentText(text)
                .setStyle(Notification.BigTextStyle().bigText(text))
                .setSmallIcon(android.R.drawable.stat_notify_error)
                .setAutoCancel(true)
                .build()
            try {
                NotificationManagerCompat.from(context).notify(alertId++, n)
            } catch (_: SecurityException) {
                // POST_NOTIFICATIONS not granted — silently skip.
            }
        }
    }
}
