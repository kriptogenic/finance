package uz.kripton.mullajiring.notifier.delivery

import android.content.Context
import android.content.pm.ServiceInfo
import androidx.work.CoroutineWorker
import androidx.work.ExistingWorkPolicy
import androidx.work.ForegroundInfo
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import uz.kripton.mullajiring.notifier.Graph
import uz.kripton.mullajiring.notifier.notify.Notifications
import java.util.concurrent.TimeUnit

/**
 * Foreground worker that drains the retry queue. A persistent notification keeps it
 * alive under background restrictions; it reschedules itself for the next due item.
 */
class DeliveryWorker(
    context: Context,
    params: WorkerParameters,
) : CoroutineWorker(context, params) {

    override suspend fun getForegroundInfo(): ForegroundInfo = ForegroundInfo(
        Notifications.SERVICE_NOTIFICATION_ID,
        Notifications.serviceNotification(applicationContext),
        ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC,
    )

    override suspend fun doWork(): Result {
        // Best-effort foreground; if the OS denies promotion we still drain in the background.
        runCatching { setForeground(getForegroundInfo()) }

        val repo = Graph.repository(applicationContext)
        val result = repo.drainOutbox()
        result.events.forEach { Notifications.notify(applicationContext, it) }

        result.nextAttemptAt?.let { next ->
            val delay = (next - System.currentTimeMillis()).coerceAtLeast(0)
            schedule(applicationContext, delay)
        }
        return Result.success()
    }

    companion object {
        private const val WORK_NAME = "delivery"

        /** Enqueue a drain pass after [delayMs]; replaces any pending pass. */
        fun schedule(context: Context, delayMs: Long = 0) {
            val request = OneTimeWorkRequestBuilder<DeliveryWorker>()
                .setInitialDelay(delayMs, TimeUnit.MILLISECONDS)
                .build()
            WorkManager.getInstance(context)
                .enqueueUniqueWork(WORK_NAME, ExistingWorkPolicy.REPLACE, request)
        }
    }
}
