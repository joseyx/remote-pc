package com.remotepc.app

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.runtime.Composable
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.navigation.navArgument
import com.remotepc.app.ui.sessions.SessionListScreen
import com.remotepc.app.ui.settings.SettingsScreen
import com.remotepc.app.ui.terminal.TerminalScreen
import com.remotepc.app.ui.theme.RemotePCTheme
import java.net.URLDecoder

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        android.webkit.WebView.setWebContentsDebuggingEnabled(true)
        handleOrbitIntent(intent)
        setContent {
            RemotePCTheme {
                AppNavigation()
            }
        }
    }

    override fun onNewIntent(intent: android.content.Intent) {
        super.onNewIntent(intent)
        handleOrbitIntent(intent)
    }

    private fun handleOrbitIntent(intent: android.content.Intent?) {
        intent?.data?.let { uri ->
            if (uri.scheme == "orbit") {
                val host = uri.host
                val port = if (uri.port != -1) uri.port.toString() else "8080"
                val token = uri.getQueryParameter("token") ?: ""
                if (!host.isNullOrBlank()) {
                    val config = com.remotepc.app.data.network.AppConfig(this)
                    config.host = host
                    config.port = port
                    config.token = token
                }
            }
        }
    }
}

@Composable
fun AppNavigation() {
    val navController = rememberNavController()

    NavHost(navController = navController, startDestination = "sessions") {
        composable("sessions") {
            SessionListScreen(
                onOpenSession = { session ->
                    val encodedName = java.net.URLEncoder.encode(session.name, "UTF-8")
                    navController.navigate("terminal/${session.id}/$encodedName")
                },
                onOpenSettings = {
                    navController.navigate("settings")
                }
            )
        }

        composable(
            route = "terminal/{sessionId}/{sessionName}",
            arguments = listOf(
                navArgument("sessionId") { type = NavType.StringType },
                navArgument("sessionName") { type = NavType.StringType }
            )
        ) { backStackEntry ->
            val sessionId = backStackEntry.arguments?.getString("sessionId") ?: ""
            val rawName = backStackEntry.arguments?.getString("sessionName") ?: "Terminal"
            val sessionName = try {
                URLDecoder.decode(rawName, "UTF-8")
            } catch (e: Exception) {
                rawName
            }

            TerminalScreen(
                sessionId = sessionId,
                sessionName = sessionName,
                onBack = { navController.popBackStack() }
            )
        }

        composable("settings") {
            SettingsScreen(
                onBack = { navController.popBackStack() }
            )
        }
    }
}
