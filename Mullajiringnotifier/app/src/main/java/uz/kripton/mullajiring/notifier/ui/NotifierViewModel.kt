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
    var senderName: String = config.senderName

    val isConfigured: Boolean get() = config.isConfigured

    fun saveSettings(baseUrl: String, token: String, senderName: String) {
        config.ingestBaseUrl = baseUrl
        config.ingestToken = token
        config.senderName = senderName
        this.baseUrl = config.ingestBaseUrl
        this.token = config.ingestToken
        this.senderName = config.senderName
    }

    fun dismissFailure(externalId: String) = viewModelScope.launch(Dispatchers.IO) {
        repo.dismissParseFailure(externalId)
    }

    fun retryNow() = DeliveryWorker.schedule(getApplication())
}
