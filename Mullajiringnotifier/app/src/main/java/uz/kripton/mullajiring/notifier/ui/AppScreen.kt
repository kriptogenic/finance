package uz.kripton.mullajiring.notifier.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Badge
import androidx.compose.material3.BadgedBox
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import uz.kripton.mullajiring.notifier.data.OutboxEntity
import uz.kripton.mullajiring.notifier.data.OutboxStatus
import uz.kripton.mullajiring.notifier.data.ParseFailureEntity

private enum class Tab { STATUS, FAILURES, SETTINGS }

@Composable
fun AppScreen(vm: NotifierViewModel = viewModel()) {
    var tab by rememberSaveable { mutableStateOf(Tab.STATUS) }
    val failureCount by vm.parseFailureCount.collectAsStateWithLifecycle()

    Scaffold(
        bottomBar = {
            NavigationBar {
                NavigationBarItem(
                    selected = tab == Tab.STATUS,
                    onClick = { tab = Tab.STATUS },
                    icon = { Text("◎") },
                    label = { Text("Status") },
                )
                NavigationBarItem(
                    selected = tab == Tab.FAILURES,
                    onClick = { tab = Tab.FAILURES },
                    icon = {
                        BadgedBox(badge = { if (failureCount > 0) Badge { Text("$failureCount") } }) {
                            Text("⚠")
                        }
                    },
                    label = { Text("Failures") },
                )
                NavigationBarItem(
                    selected = tab == Tab.SETTINGS,
                    onClick = { tab = Tab.SETTINGS },
                    icon = { Text("⚙") },
                    label = { Text("Settings") },
                )
            }
        },
    ) { padding ->
        Column(Modifier.fillMaxSize().padding(padding)) {
            when (tab) {
                Tab.STATUS -> StatusScreen(vm)
                Tab.FAILURES -> FailuresScreen(vm)
                Tab.SETTINGS -> SettingsScreen(vm)
            }
        }
    }
}

@Composable
private fun StatusScreen(vm: NotifierViewModel) {
    val pending by vm.pendingCount.collectAsStateWithLifecycle()
    val failed by vm.failedCount.collectAsStateWithLifecycle()
    val recent by vm.recentOutbox.collectAsStateWithLifecycle()

    Column(Modifier.fillMaxSize().padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
        if (!vm.isConfigured) {
            Card(Modifier.fillMaxWidth()) {
                Text(
                    "Not configured — set the ingest endpoint and token in Settings.",
                    Modifier.padding(16.dp),
                    color = MaterialTheme.colorScheme.error,
                )
            }
        }
        Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            StatCard("Pending", pending.toString(), Modifier.weight(1f))
            StatCard("Failed", failed.toString(), Modifier.weight(1f))
        }
        Button(onClick = { vm.retryNow() }) { Text("Retry now") }

        Text("Recent transactions", style = MaterialTheme.typography.titleMedium)
        LazyColumn(verticalArrangement = Arrangement.spacedBy(6.dp)) {
            items(recent) { OutboxRow(it) }
        }
    }
}

@Composable
private fun StatCard(label: String, value: String, modifier: Modifier = Modifier) {
    Card(modifier) {
        Column(Modifier.padding(16.dp)) {
            Text(value, style = MaterialTheme.typography.headlineMedium)
            Text(label, style = MaterialTheme.typography.bodyMedium)
        }
    }
}

@Composable
private fun OutboxRow(item: OutboxEntity) {
    Card(Modifier.fillMaxWidth()) {
        Column(Modifier.padding(12.dp)) {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                Text(item.merchant, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
                Text(formatMinor(item.amountMinor, item.currency))
            }
            Text(
                "${item.occurredAtUtc} • ${statusLabel(item.status)}",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            item.lastError?.let {
                Text(it, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.error)
            }
        }
    }
}

@Composable
private fun FailuresScreen(vm: NotifierViewModel) {
    val failures by vm.parseFailures.collectAsStateWithLifecycle()
    Column(Modifier.fillMaxSize().padding(16.dp)) {
        Text("Unparsed bank messages", style = MaterialTheme.typography.titleMedium)
        if (failures.isEmpty()) {
            Text("Nothing to review.", Modifier.padding(top = 12.dp))
        }
        LazyColumn(verticalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.padding(top = 12.dp)) {
            items(failures) { FailureRow(it) { vm.dismissFailure(it.externalId) } }
        }
    }
}

@Composable
private fun FailureRow(item: ParseFailureEntity, onDismiss: () -> Unit) {
    Card(Modifier.fillMaxWidth()) {
        Column(Modifier.padding(12.dp)) {
            Text(item.rawBody, style = MaterialTheme.typography.bodyMedium)
            Text(item.reason, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.error)
            TextButton(onClick = onDismiss) { Text("Dismiss") }
        }
    }
}

@Composable
private fun SettingsScreen(vm: NotifierViewModel) {
    var baseUrl by rememberSaveable { mutableStateOf(vm.baseUrl) }
    var token by rememberSaveable { mutableStateOf(vm.token) }
    var card by rememberSaveable { mutableStateOf(vm.card) }
    var saved by remember { mutableStateOf(false) }

    Column(
        Modifier.fillMaxSize().padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Text("Ingest configuration", style = MaterialTheme.typography.titleMedium)
        OutlinedTextField(
            value = baseUrl,
            onValueChange = { baseUrl = it; saved = false },
            label = { Text("Ingest base URL") },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true,
        )
        OutlinedTextField(
            value = token,
            onValueChange = { token = it; saved = false },
            label = { Text("Ingest token (bearer)") },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true,
        )
        OutlinedTextField(
            value = card,
            onValueChange = { card = it; saved = false },
            label = { Text("Card (masked)") },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true,
        )
        Button(onClick = { vm.saveSettings(baseUrl, token, card); saved = true }) { Text("Save") }
        if (saved) Text("Saved.", color = MaterialTheme.colorScheme.primary)

        HorizontalDivider()
        Text("Maintenance", style = MaterialTheme.typography.titleMedium)
        OutlinedButton(onClick = { vm.runBackfill() }) { Text("Backfill inbox now") }
    }
}

private fun statusLabel(status: OutboxStatus) = when (status) {
    OutboxStatus.PENDING -> "pending"
    OutboxStatus.DELIVERED -> "delivered"
    OutboxStatus.FAILED -> "failed"
}

/** minor units -> grouped string, e.g. 2850000 UZS -> "28 500.00 UZS". */
private fun formatMinor(minor: Long, currency: String): String {
    val sign = if (minor < 0) "-" else ""
    val abs = kotlin.math.abs(minor)
    val whole = abs / 100
    val frac = (abs % 100).toString().padStart(2, '0')
    val grouped = whole.toString().reversed().chunked(3).joinToString(" ").reversed()
    return "$sign$grouped.$frac $currency"
}
