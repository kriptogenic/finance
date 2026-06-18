package uz.kripton.mullajiring.notifier.sms

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import uz.kripton.mullajiring.notifier.delivery.DeliveryWorker

/** After reboot, resume draining any persisted retry-queue items. */
class BootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action == Intent.ACTION_BOOT_COMPLETED) {
            DeliveryWorker.schedule(context.applicationContext)
        }
    }
}
