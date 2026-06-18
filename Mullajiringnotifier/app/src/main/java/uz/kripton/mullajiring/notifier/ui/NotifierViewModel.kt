package uz.kripton.mullajiring.notifier.ui

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch
import uz.kripton.mullajiring.notifier.Graph
import uz.kripton.mullajiring.notifier.data.OutboxEntity
import uz.kripton.mullajiring.notifier.data.ParseFailureEntity
import uz.kripton.mullajiring.notifier.delivery.DeliveryWorker
import uz.kripton.mullajiring.notifier.sms.SmsBackfill

class NotifierViewModel(app: Application) : AndroidViewModel(app) {

    private val config = Graph.config(app)
    private val repo = Graph.repository(app)

    val pendingCount = repo.pendingCount.stateIn(viewModelScope, SharingStarted.WhileSubscribed(5000), 0)
    val failedCount = repo.failedCount.stateIn(viewModelScope, SharingStarted.WhileSubscribed(5000), 0)
    val parseFailureCount =
        repo.activeParseFailureCount.stateIn(viewModelScope, SharingStarted.WhileSubscribed(5000), 0)
    val parseFailures =
        repo.activeParseFailures.stateIn(viewModelScope, SharingStarted.WhileSubscribed(5000), emptyList<ParseFailureEntity>())
    val recentOutbox =
        repo.recentOutbox.stateIn(viewModelScope, SharingStarted.WhileSubscribed(5000), emptyList<OutboxEntity>())

    // Settings are read once into editable state; persisted on save.
    var baseUrl: String = config.ingestBaseUrl
    var token: String = config.ingestToken
    var card: String = config.knownCard

    val isConfigured: Boolean get() = config.isConfigured

    fun saveSettings(baseUrl: String, token: String, card: String) {
        config.ingestBaseUrl = baseUrl
        config.ingestToken = token
        config.knownCard = card
        this.baseUrl = config.ingestBaseUrl
        this.token = config.ingestToken
        this.card = config.knownCard
    }

    fun dismissFailure(externalId: String) = viewModelScope.launch(Dispatchers.IO) {
        repo.dismissParseFailure(externalId)
    }

    fun runBackfill() = viewModelScope.launch(Dispatchers.IO) {
        SmsBackfill.run(getApplication())
    }

    fun retryNow() = DeliveryWorker.schedule(getApplication())
}
