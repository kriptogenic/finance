package uz.kripton.mullajiring.notifier

import android.Manifest
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import uz.kripton.mullajiring.notifier.sms.SmsBackfill
import uz.kripton.mullajiring.notifier.ui.AppScreen
import uz.kripton.mullajiring.notifier.ui.theme.MullajiringNotifierTheme
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch

class MainActivity : ComponentActivity() {

    private val permissionLauncher =
        registerForActivityResult(ActivityResultContracts.RequestMultiplePermissions()) { grants ->
            // With SMS read access, backfill the inbox once.
            if (grants[Manifest.permission.READ_SMS] == true) {
                CoroutineScope(Dispatchers.IO).launch { SmsBackfill.run(applicationContext) }
            }
        }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        requestPermissions()
        setContent {
            MullajiringNotifierTheme {
                AppScreen()
            }
        }
    }

    private fun requestPermissions() {
        permissionLauncher.launch(
            arrayOf(
                Manifest.permission.RECEIVE_SMS,
                Manifest.permission.READ_SMS,
                Manifest.permission.POST_NOTIFICATIONS,
            ),
        )
    }
}
