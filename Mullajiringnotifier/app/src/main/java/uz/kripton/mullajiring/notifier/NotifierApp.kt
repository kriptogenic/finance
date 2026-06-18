package uz.kripton.mullajiring.notifier

import android.app.Application
import uz.kripton.mullajiring.notifier.delivery.DeliveryWorker
import uz.kripton.mullajiring.notifier.notify.Notifications

class NotifierApp : Application() {
    override fun onCreate() {
        super.onCreate()
        Notifications.ensureChannels(this)
        // Resume any persisted retry-queue work left over from a previous process.
        DeliveryWorker.schedule(this)
    }
}
