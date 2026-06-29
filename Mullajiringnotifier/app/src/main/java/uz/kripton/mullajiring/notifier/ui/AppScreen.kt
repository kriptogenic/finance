package uz.kripton.mullajiring.notifier.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Badge
import androidx.compose.material3.BadgedBox
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.provider.Settings
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.lifecycle.compose.LocalLifecycleOwner
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
            items(recent) { OutboxRow(it, onResend = { vm.resend(it.externalId) }) }
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
private fun OutboxRow(item: OutboxEntity, onResend: () -> Unit) {
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
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                TextButton(onClick = onResend) { Text("Resend") }
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
    var senderName by rememberSaveable { mutableStateOf(vm.senderName) }
    var pushPackage by rememberSaveable { mutableStateOf(vm.pushPackage) }
    var tokenVisible by rememberSaveable { mutableStateOf(false) }
    var saved by remember { mutableStateOf(false) }
    val smsGranted = rememberSmsPermissionGranted()
    val pushGranted = rememberNotificationAccessGranted()
    val healthResult by vm.healthResult.collectAsStateWithLifecycle()

    Column(
        Modifier.fillMaxSize().padding(16.dp).verticalScroll(rememberScrollState()),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        PermissionStatusCard(smsGranted)
        NotificationAccessCard(pushGranted)

        Text("Ingest configuration", style = MaterialTheme.typography.titleMedium)
        OutlinedTextField(
            value = senderName,
            onValueChange = { senderName = it; saved = false },
            label = { Text("SMS sender name") },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true,
        )
        OutlinedTextField(
            value = pushPackage,
            onValueChange = { pushPackage = it; saved = false },
            label = { Text("Bank app package (push)") },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true,
        )
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
            visualTransformation = if (tokenVisible) VisualTransformation.None else PasswordVisualTransformation(),
            trailingIcon = {
                if (token.isNotEmpty()) {
                    TextButton(onClick = { tokenVisible = !tokenVisible }) {
                        Text(if (tokenVisible) "Hide" else "Show")
                    }
                }
            },
        )
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Button(onClick = {
                vm.saveSettings(baseUrl, token, senderName, pushPackage)
                tokenVisible = false // hide the token again after saving
                saved = true
            }) { Text("Save") }
            TextButton(onClick = { vm.testConnection(baseUrl) }) { Text("Test connection") }
        }
        if (saved) Text("Saved.", color = MaterialTheme.colorScheme.primary)
        healthResult?.let {
            val ok = it.startsWith("OK")
            Text(
                "Health: $it",
                style = MaterialTheme.typography.bodySmall,
                color = if (ok) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.error,
            )
        }
    }
}

/** Reads RECEIVE_SMS grant state, refreshing whenever the screen resumes. */
@Composable
private fun rememberSmsPermissionGranted(): Boolean {
    val context = LocalContext.current
    val lifecycleOwner = LocalLifecycleOwner.current
    var granted by remember {
        mutableStateOf(
            ContextCompat.checkSelfPermission(context, Manifest.permission.RECEIVE_SMS) ==
                PackageManager.PERMISSION_GRANTED,
        )
    }
    DisposableEffect(lifecycleOwner) {
        val observer = LifecycleEventObserver { _, event ->
            if (event == Lifecycle.Event.ON_RESUME) {
                granted = ContextCompat.checkSelfPermission(context, Manifest.permission.RECEIVE_SMS) ==
                    PackageManager.PERMISSION_GRANTED
            }
        }
        lifecycleOwner.lifecycle.addObserver(observer)
        onDispose { lifecycleOwner.lifecycle.removeObserver(observer) }
    }
    return granted
}

@Composable
private fun PermissionStatusCard(granted: Boolean) {
    val context = LocalContext.current
    val launcher = rememberLauncherForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { /* status refreshes on resume */ }

    Card(Modifier.fillMaxWidth()) {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text("SMS reception", style = MaterialTheme.typography.titleSmall)
            Text(
                if (granted) "RECEIVE_SMS: granted" else "RECEIVE_SMS: not granted",
                color = if (granted) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.error,
            )
            if (!granted) {
                Text(
                    "Without this permission no bank messages can be received.",
                    style = MaterialTheme.typography.bodySmall,
                )
            }
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                if (!granted) {
                    Button(onClick = { launcher.launch(Manifest.permission.RECEIVE_SMS) }) {
                        Text("Grant permission")
                    }
                }
                TextButton(onClick = { context.openAppSettings() }) {
                    Text("Open app settings")
                }
            }
        }
    }
}

private fun android.content.Context.openAppSettings() {
    startActivity(
        Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS, Uri.fromParts("package", packageName, null))
            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK),
    )
}

/** Whether this app is an enabled notification listener, refreshed when the screen resumes. */
@Composable
private fun rememberNotificationAccessGranted(): Boolean {
    val context = LocalContext.current
    val lifecycleOwner = LocalLifecycleOwner.current
    fun check() = NotificationManagerCompat.getEnabledListenerPackages(context).contains(context.packageName)
    var granted by remember { mutableStateOf(check()) }
    DisposableEffect(lifecycleOwner) {
        val observer = LifecycleEventObserver { _, event ->
            if (event == Lifecycle.Event.ON_RESUME) granted = check()
        }
        lifecycleOwner.lifecycle.addObserver(observer)
        onDispose { lifecycleOwner.lifecycle.removeObserver(observer) }
    }
    return granted
}

@Composable
private fun NotificationAccessCard(granted: Boolean) {
    val context = LocalContext.current
    Card(Modifier.fillMaxWidth()) {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text("Push reception", style = MaterialTheme.typography.titleSmall)
            Text(
                if (granted) "Notification access: granted" else "Notification access: not granted",
                color = if (granted) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.error,
            )
            if (!granted) {
                Text(
                    "Without notification access, bank push notifications can't be read.",
                    style = MaterialTheme.typography.bodySmall,
                )
            }
            TextButton(onClick = { context.openNotificationAccessSettings() }) {
                Text("Open notification access")
            }
        }
    }
}

private fun android.content.Context.openNotificationAccessSettings() {
    startActivity(
        Intent(Settings.ACTION_NOTIFICATION_LISTENER_SETTINGS)
            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK),
    )
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
