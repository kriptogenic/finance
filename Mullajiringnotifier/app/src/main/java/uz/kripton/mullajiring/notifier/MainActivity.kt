package uz.kripton.mullajiring.notifier

import android.Manifest
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import uz.kripton.mullajiring.notifier.ui.AppScreen
import uz.kripton.mullajiring.notifier.ui.theme.MullajiringNotifierTheme

class MainActivity : ComponentActivity() {

    private val permissionLauncher =
        registerForActivityResult(ActivityResultContracts.RequestMultiplePermissions()) { }

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
                Manifest.permission.POST_NOTIFICATIONS,
            ),
        )
    }
}
