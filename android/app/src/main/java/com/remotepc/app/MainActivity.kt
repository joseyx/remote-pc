package com.remotepc.app

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Scaffold
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.navigation.navArgument
import com.remotepc.app.data.model.VpnStatus
import com.remotepc.app.data.network.ApiClient
import com.remotepc.app.data.network.AppConfig
import com.remotepc.app.ui.bubbles.BubblesScreen
import com.remotepc.app.ui.components.OrbitBottomBar
import com.remotepc.app.ui.components.OrbitTab
import com.remotepc.app.ui.components.OrbitTopBar
import com.remotepc.app.ui.daily.DailyScreen
import com.remotepc.app.ui.monitor.MonitorScreen
import com.remotepc.app.ui.sessions.SessionListScreen
import com.remotepc.app.ui.settings.SettingsScreen
import com.remotepc.app.ui.terminal.TerminalScreen
import com.remotepc.app.ui.theme.RemotePCTheme
import com.remotepc.app.ui.theme.TerminalBackground
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import java.net.URLDecoder
import java.net.URLEncoder

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
                    val config = AppConfig(this)
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

    NavHost(navController = navController, startDestination = "dashboard") {
        composable("dashboard") {
            MainDashboardScreen(
                onOpenSession = { id, name ->
                    val encodedName = URLEncoder.encode(name, "UTF-8")
                    navController.navigate("terminal/$id/$encodedName")
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

@Composable
fun MainDashboardScreen(
    onOpenSession: (id: String, name: String) -> Unit,
    onOpenSettings: () -> Unit
) {
    val context = LocalContext.current
    val config = remember { AppConfig(context) }
    val apiClient = remember { ApiClient(config) }
    val scope = rememberCoroutineScope()

    var currentTab by remember { mutableStateOf(OrbitTab.Bubbles) }
    var isConnected by remember { mutableStateOf(false) }
    var vpnStatus by remember { mutableStateOf<VpnStatus?>(null) }
    var sessionsCount by remember { mutableIntStateOf(0) }
    var bubblesCount by remember { mutableIntStateOf(0) }

    fun refreshGlobalState() {
        scope.launch {
            isConnected = apiClient.checkHealth()
            if (isConnected) {
                try {
                    val sessList = apiClient.listSessions()
                    sessionsCount = sessList.count { it.isRunning }
                } catch (e: Exception) {
                    // ignore
                }
                try {
                    vpnStatus = apiClient.getVpnStatus()
                } catch (e: Exception) {
                    // ignore
                }
            }
        }
    }

    LaunchedEffect(Unit) {
        refreshGlobalState()
        while (true) {
            delay(10000)
            refreshGlobalState()
        }
    }

    val topBarTitle = when (currentTab) {
        OrbitTab.Bubbles -> "Workspace Tasks"
        OrbitTab.Sessions -> "Terminals & Servers"
        OrbitTab.Daily -> "Daily Standup"
        OrbitTab.Monitor -> "System Monitor"
    }

    val topBarSubtitle = when (currentTab) {
        OrbitTab.Bubbles -> "Aditum, Brif & Worktrees"
        OrbitTab.Sessions -> "$sessionsCount active sessions"
        OrbitTab.Daily -> "Handoffs & Action Items"
        OrbitTab.Monitor -> "Resource & VPN Status"
    }

    Scaffold(
        topBar = {
            OrbitTopBar(
                title = topBarTitle,
                subtitle = topBarSubtitle,
                isConnected = isConnected,
                vpnStatus = vpnStatus,
                onRefresh = { refreshGlobalState() },
                onOpenSettings = onOpenSettings
            )
        },
        bottomBar = {
            OrbitBottomBar(
                currentTab = currentTab,
                onTabSelected = { currentTab = it },
                sessionsCount = sessionsCount,
                bubblesCount = bubblesCount
            )
        },
        containerColor = TerminalBackground
    ) { padding ->
        Box(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .background(TerminalBackground)
        ) {
            when (currentTab) {
                OrbitTab.Bubbles -> {
                    BubblesScreen(
                        apiClient = apiClient,
                        onOpenSession = onOpenSession
                    )
                }
                OrbitTab.Sessions -> {
                    SessionListScreen(
                        apiClient = apiClient,
                        onOpenSession = { sess -> onOpenSession(sess.id, sess.name) }
                    )
                }
                OrbitTab.Daily -> {
                    DailyScreen(
                        apiClient = apiClient
                    )
                }
                OrbitTab.Monitor -> {
                    MonitorScreen(
                        apiClient = apiClient
                    )
                }
            }
        }
    }
}
