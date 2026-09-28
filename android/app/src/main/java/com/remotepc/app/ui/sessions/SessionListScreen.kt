package com.remotepc.app.ui.sessions

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.Folder
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.KeyboardArrowUp
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.filled.Terminal
import androidx.compose.material.icons.filled.Tune
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.ui.text.TextStyle
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.remotepc.app.data.model.Session
import com.remotepc.app.data.network.ApiClient
import com.remotepc.app.data.network.AppConfig
import com.remotepc.app.ui.dialogs.CreateSessionDialog
import com.remotepc.app.ui.theme.TerminalAccent
import com.remotepc.app.ui.theme.TerminalBackground
import com.remotepc.app.ui.theme.TerminalGreen
import com.remotepc.app.ui.theme.TerminalRed
import com.remotepc.app.ui.theme.TerminalSurface
import com.remotepc.app.ui.theme.TerminalSurfaceVariant
import com.remotepc.app.ui.theme.TerminalText
import com.remotepc.app.ui.theme.TerminalTextMuted
import com.remotepc.app.data.model.VpnStatus
import com.remotepc.app.ui.theme.TerminalBorder
import com.remotepc.app.ui.theme.TerminalTextDim
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SessionListScreen(
    onOpenSession: (Session) -> Unit,
    onOpenSettings: () -> Unit
) {
    val context = LocalContext.current
    val config = remember { AppConfig(context) }
    val apiClient = remember { ApiClient(config) }
    val scope = rememberCoroutineScope()

    var sessions by remember { mutableStateOf<List<Session>>(emptyList()) }
    var isLoading by remember { mutableStateOf(false) }
    var errorMessage by remember { mutableStateOf<String?>(null) }
    var showCreateDialog by remember { mutableStateOf(false) }

    var vpnStatus by remember { mutableStateOf<VpnStatus?>(null) }
    var isVpnLoading by remember { mutableStateOf(false) }

    fun refreshSessions() {
        scope.launch {
            isLoading = true
            errorMessage = null
            try {
                sessions = apiClient.listSessions()
            } catch (e: Exception) {
                errorMessage = "Failed to load sessions. Is server running?"
            } finally {
                isLoading = false
            }
        }
        scope.launch {
            try {
                vpnStatus = apiClient.getVpnStatus()
            } catch (e: Exception) {
                // Ignore or keep previous
            }
        }
    }

    fun connectVpn(location: String = "", protocol: String = "") {
        scope.launch {
            isVpnLoading = true
            try {
                apiClient.connectVpn(location = location, protocol = protocol)
                for (i in 1..6) {
                    delay(2000)
                    val updated = apiClient.getVpnStatus()
                    vpnStatus = updated
                    if (updated.connected) {
                        break
                    }
                }
            } catch (e: Exception) {
                // handle error
            } finally {
                isVpnLoading = false
            }
        }
    }

    fun disconnectVpn() {
        scope.launch {
            isVpnLoading = true
            try {
                apiClient.disconnectVpn()
                for (i in 1..6) {
                    delay(2000)
                    val updated = apiClient.getVpnStatus()
                    vpnStatus = updated
                    if (!updated.connected) {
                        break
                    }
                }
            } catch (e: Exception) {
                // handle error
            } finally {
                isVpnLoading = false
            }
        }
    }

    LaunchedEffect(Unit) {
        refreshSessions()
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Icon(
                            imageVector = Icons.Default.Terminal,
                            contentDescription = null,
                            tint = TerminalAccent,
                            modifier = Modifier.size(24.dp)
                        )
                        Spacer(modifier = Modifier.width(8.dp))
                        Text("Remote PC Sessions", color = TerminalText, fontWeight = FontWeight.Bold)
                    }
                },
                actions = {
                    IconButton(onClick = { refreshSessions() }) {
                        Icon(Icons.Default.Refresh, contentDescription = "Refresh", tint = TerminalText)
                    }
                    IconButton(onClick = onOpenSettings) {
                        Icon(Icons.Default.Settings, contentDescription = "Settings", tint = TerminalText)
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = TerminalSurface)
            )
        },
        floatingActionButton = {
            FloatingActionButton(
                onClick = { showCreateDialog = true },
                containerColor = TerminalAccent,
                contentColor = TerminalSurface
            ) {
                Icon(Icons.Default.Add, contentDescription = "New Session")
            }
        },
        containerColor = TerminalBackground
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .padding(12.dp)
        ) {
                if (vpnStatus != null && vpnStatus!!.installed) {
                    VpnCard(
                        vpn = vpnStatus!!,
                        isLoading = isVpnLoading,
                        onConnect = { loc, proto -> connectVpn(loc, proto) },
                        onDisconnect = { disconnectVpn() }
                    )
                    Spacer(modifier = Modifier.height(10.dp))
                }

                when {
                    isLoading && sessions.isEmpty() -> {
                        Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                            CircularProgressIndicator(color = TerminalAccent)
                        }
                    }
                    errorMessage != null -> {
                        Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                                Text(text = errorMessage ?: "", color = TerminalRed, fontSize = 14.sp)
                                Spacer(modifier = Modifier.height(12.dp))
                                IconButton(onClick = { refreshSessions() }) {
                                    Icon(Icons.Default.Refresh, contentDescription = "Retry", tint = TerminalAccent)
                                }
                            }
                        }
                    }
                    sessions.isEmpty() -> {
                        Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                            Column(
                                modifier = Modifier.padding(24.dp),
                                horizontalAlignment = Alignment.CenterHorizontally
                            ) {
                                Icon(
                                    imageVector = Icons.Default.Terminal,
                                    contentDescription = null,
                                    tint = TerminalTextMuted,
                                    modifier = Modifier.size(54.dp)
                                )
                                Spacer(modifier = Modifier.height(12.dp))
                                Text(
                                    text = "No Active Sessions",
                                    color = TerminalText,
                                    style = MaterialTheme.typography.titleMedium
                                )
                                Spacer(modifier = Modifier.height(6.dp))
                                Text(
                                    text = "Tap the + button to launch Claude Code or a terminal in any PC folder.",
                                    color = TerminalTextMuted,
                                    fontSize = 13.sp,
                                    textAlign = androidx.compose.ui.text.style.TextAlign.Center
                                )
                            }
                        }
                    }
                    else -> {
                        LazyColumn(
                            modifier = Modifier.fillMaxSize(),
                            verticalArrangement = Arrangement.spacedBy(10.dp)
                        ) {
                            items(sessions) { session ->
                                SessionCard(
                                    session = session,
                                    onClick = { onOpenSession(session) },
                                    onDelete = {
                                        scope.launch {
                                            apiClient.deleteSession(session.id)
                                            refreshSessions()
                                        }
                                    }
                                )
                            }
                        }
                    }
                }
            }
    }

    if (showCreateDialog) {
        CreateSessionDialog(
            apiClient = apiClient,
            onDismiss = { showCreateDialog = false },
            onCreated = { req ->
                showCreateDialog = false
                scope.launch {
                    try {
                        val created = apiClient.createSession(req)
                        refreshSessions()
                        onOpenSession(created)
                    } catch (e: Exception) {
                        errorMessage = "Error creating session: ${e.localizedMessage}"
                    }
                }
            }
        )
    }
}

@Composable
private fun SessionCard(
    session: Session,
    onClick: () -> Unit,
    onDelete: () -> Unit
) {
    Card(
        modifier = Modifier
            .fillMaxWidth()
            .clickable { onClick() },
        shape = RoundedCornerShape(12.dp),
        colors = CardDefaults.cardColors(containerColor = TerminalSurface)
    ) {
        Column(modifier = Modifier.padding(14.dp)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        text = session.name,
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.Bold,
                        color = TerminalText
                    )
                    Spacer(modifier = Modifier.width(8.dp))
                    Surface(
                        color = if (session.isRunning) TerminalGreen.copy(alpha = 0.2f) else TerminalRed.copy(alpha = 0.2f),
                        shape = RoundedCornerShape(4.dp)
                    ) {
                        Text(
                            text = if (session.isRunning) "RUNNING" else "STOPPED",
                            fontSize = 10.sp,
                            fontWeight = FontWeight.Bold,
                            color = if (session.isRunning) TerminalGreen else TerminalRed,
                            modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp)
                        )
                    }
                }

                IconButton(onClick = onDelete, modifier = Modifier.size(24.dp)) {
                    Icon(Icons.Default.Close, contentDescription = "Terminate", tint = TerminalTextMuted)
                }
            }

            Spacer(modifier = Modifier.height(6.dp))

            Row(verticalAlignment = Alignment.CenterVertically) {
                Icon(
                    imageVector = Icons.Default.Folder,
                    contentDescription = null,
                    tint = TerminalAccent,
                    modifier = Modifier.size(16.dp)
                )
                Spacer(modifier = Modifier.width(6.dp))
                Text(
                    text = session.cwd.ifEmpty { "Default Directory" },
                    fontSize = 12.sp,
                    fontFamily = FontFamily.Monospace,
                    color = TerminalTextMuted
                )
            }

            Spacer(modifier = Modifier.height(4.dp))

            Surface(
                color = TerminalSurfaceVariant,
                shape = RoundedCornerShape(4.dp)
            ) {
                Text(
                    text = "> ${session.command}",
                    fontFamily = FontFamily.Monospace,
                    fontSize = 11.sp,
                    color = TerminalAccent,
                    modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp)
                )
            }
        }
    }
}

@Composable
private fun VpnCard(
    vpn: VpnStatus,
    isLoading: Boolean,
    onConnect: (location: String, protocol: String) -> Unit,
    onDisconnect: () -> Unit
) {
    val isConnected = vpn.connected
    val stateColor = if (isConnected) TerminalGreen else TerminalTextDim
    var expanded by remember { mutableStateOf(false) }

    var selectedLocation by remember { mutableStateOf("best") }
    var customLocation by remember { mutableStateOf("") }
    var selectedProtocol by remember { mutableStateOf("wireguard") }

    val locations = listOf(
        "best" to "⭐ Best",
        "Miami" to "Miami",
        "New York" to "New York",
        "Dallas" to "Dallas",
        "Chicago" to "Chicago",
        "Los Angeles" to "Los Angeles",
        "Seattle" to "Seattle",
        "Toronto" to "Toronto",
        "London" to "London",
        "Frankfurt" to "Frankfurt",
        "Paris" to "Paris",
        "Amsterdam" to "Amsterdam",
        "custom" to "✏️ Custom..."
    )

    val protocols = listOf(
        "wireguard" to "WireGuard",
        "stealth" to "Stealth (443)",
        "udp" to "OpenVPN UDP",
        "tcp" to "OpenVPN TCP",
        "wstunnel" to "WSTunnel",
        "ikev2" to "IKEv2"
    )

    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(containerColor = TerminalSurface),
        shape = RoundedCornerShape(10.dp),
        border = BorderStroke(1.dp, if (isConnected) TerminalGreen.copy(alpha = 0.35f) else TerminalBorder)
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp)
        ) {
            // Header Row
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    modifier = Modifier
                        .weight(1f)
                        .clickable { expanded = !expanded }
                ) {
                    Box(
                        modifier = Modifier
                            .size(10.dp)
                            .background(if (isConnected) TerminalGreen else TerminalTextDim, CircleShape)
                    )
                    Spacer(modifier = Modifier.width(10.dp))
                    Column {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(
                                text = "Windscribe VPN",
                                color = TerminalText,
                                fontWeight = FontWeight.Bold,
                                fontSize = 13.sp
                            )
                            Spacer(modifier = Modifier.width(6.dp))
                            Text(
                                text = if (isConnected) "CONNECTED" else "DISCONNECTED",
                                color = stateColor,
                                fontWeight = FontWeight.Bold,
                                fontSize = 10.sp
                            )
                        }
                        val subtitle = if (isConnected) {
                            val p = if (vpn.protocol.isNotEmpty()) " · ${vpn.protocol}" else ""
                            "${vpn.location.ifEmpty { "Connected" }} · ${vpn.ip.ifEmpty { "Secure" }}$p"
                        } else {
                            "Direct connection · Ready"
                        }
                        Text(
                            text = subtitle,
                            color = TerminalTextMuted,
                            fontSize = 11.sp
                        )
                    }
                }

                Row(verticalAlignment = Alignment.CenterVertically) {
                    IconButton(
                        onClick = { expanded = !expanded },
                        modifier = Modifier.size(32.dp)
                    ) {
                        Icon(
                            imageVector = if (expanded) Icons.Default.KeyboardArrowUp else Icons.Default.Tune,
                            contentDescription = "Options",
                            tint = if (expanded) TerminalAccent else TerminalTextDim,
                            modifier = Modifier.size(18.dp)
                        )
                    }

                    Spacer(modifier = Modifier.width(4.dp))

                    if (isLoading) {
                        CircularProgressIndicator(
                            modifier = Modifier.size(20.dp),
                            color = TerminalAccent,
                            strokeWidth = 2.dp
                        )
                    } else if (isConnected) {
                        Button(
                            onClick = onDisconnect,
                            colors = ButtonDefaults.buttonColors(containerColor = TerminalRed.copy(alpha = 0.85f)),
                            contentPadding = PaddingValues(horizontal = 10.dp, vertical = 4.dp),
                            shape = RoundedCornerShape(6.dp)
                        ) {
                            Text("Disconnect", fontSize = 11.sp, color = TerminalSurface)
                        }
                    } else {
                        Button(
                            onClick = {
                                val loc = if (selectedLocation == "custom") customLocation.trim() else selectedLocation
                                onConnect(loc, selectedProtocol)
                            },
                            colors = ButtonDefaults.buttonColors(containerColor = TerminalAccent),
                            contentPadding = PaddingValues(horizontal = 12.dp, vertical = 4.dp),
                            shape = RoundedCornerShape(6.dp)
                        ) {
                            Text("Connect", fontSize = 11.sp, color = TerminalSurface)
                        }
                    }
                }
            }

            // Expanded Settings Section
            if (expanded) {
                Spacer(modifier = Modifier.height(10.dp))
                HorizontalDivider(color = TerminalBorder, thickness = 1.dp)
                Spacer(modifier = Modifier.height(8.dp))

                // Location selector
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text(
                        text = "LOCATION",
                        color = TerminalTextDim,
                        fontSize = 10.sp,
                        fontWeight = FontWeight.Bold,
                        letterSpacing = 0.5.sp
                    )
                    Text(
                        text = if (selectedLocation == "custom") customLocation.ifEmpty { "Custom" } else selectedLocation,
                        color = TerminalAccent,
                        fontSize = 11.sp,
                        fontWeight = FontWeight.SemiBold
                    )
                }
                Spacer(modifier = Modifier.height(6.dp))
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .horizontalScroll(rememberScrollState()),
                    horizontalArrangement = Arrangement.spacedBy(6.dp)
                ) {
                    locations.forEach { (id, label) ->
                        val isSelected = selectedLocation == id
                        Surface(
                            shape = RoundedCornerShape(14.dp),
                            color = if (isSelected) TerminalAccent.copy(alpha = 0.2f) else TerminalSurfaceVariant,
                            border = BorderStroke(
                                1.dp,
                                if (isSelected) TerminalAccent else TerminalBorder
                            ),
                            modifier = Modifier.clickable { selectedLocation = id }
                        ) {
                            Text(
                                text = label,
                                fontSize = 11.sp,
                                color = if (isSelected) TerminalAccent else TerminalText,
                                modifier = Modifier.padding(horizontal = 10.dp, vertical = 5.dp)
                            )
                        }
                    }
                }

                if (selectedLocation == "custom") {
                    Spacer(modifier = Modifier.height(6.dp))
                    OutlinedTextField(
                        value = customLocation,
                        onValueChange = { customLocation = it },
                        placeholder = { Text("City, region or ISO code (e.g. Zurich, US)", fontSize = 11.sp, color = TerminalTextDim) },
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth(),
                        colors = OutlinedTextFieldDefaults.colors(
                            focusedBorderColor = TerminalAccent,
                            unfocusedBorderColor = TerminalBorder,
                            focusedTextColor = TerminalText,
                            unfocusedTextColor = TerminalText,
                            cursorColor = TerminalAccent
                        ),
                        textStyle = TextStyle(fontSize = 12.sp)
                    )
                }

                Spacer(modifier = Modifier.height(10.dp))

                // Protocol selector
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text(
                        text = "PROTOCOL",
                        color = TerminalTextDim,
                        fontSize = 10.sp,
                        fontWeight = FontWeight.Bold,
                        letterSpacing = 0.5.sp
                    )
                    Text(
                        text = selectedProtocol.uppercase(),
                        color = TerminalAccent,
                        fontSize = 11.sp,
                        fontWeight = FontWeight.SemiBold
                    )
                }
                Spacer(modifier = Modifier.height(6.dp))
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .horizontalScroll(rememberScrollState()),
                    horizontalArrangement = Arrangement.spacedBy(6.dp)
                ) {
                    protocols.forEach { (id, label) ->
                        val isSelected = selectedProtocol == id
                        Surface(
                            shape = RoundedCornerShape(14.dp),
                            color = if (isSelected) TerminalAccent.copy(alpha = 0.2f) else TerminalSurfaceVariant,
                            border = BorderStroke(
                                1.dp,
                                if (isSelected) TerminalAccent else TerminalBorder
                            ),
                            modifier = Modifier.clickable { selectedProtocol = id }
                        ) {
                            Text(
                                text = label,
                                fontSize = 11.sp,
                                color = if (isSelected) TerminalAccent else TerminalText,
                                modifier = Modifier.padding(horizontal = 10.dp, vertical = 5.dp)
                            )
                        }
                    }
                }

                // If currently connected, offer "Switch Location / Protocol" action
                if (isConnected && !isLoading) {
                    Spacer(modifier = Modifier.height(10.dp))
                    Button(
                        onClick = {
                            val loc = if (selectedLocation == "custom") customLocation.trim() else selectedLocation
                            onConnect(loc, selectedProtocol)
                        },
                        colors = ButtonDefaults.buttonColors(containerColor = TerminalSurfaceVariant),
                        border = BorderStroke(1.dp, TerminalAccent.copy(alpha = 0.5f)),
                        modifier = Modifier.fillMaxWidth(),
                        shape = RoundedCornerShape(6.dp)
                    ) {
                        Text(
                            text = "⚡ Apply & Switch to $selectedLocation ($selectedProtocol)",
                            fontSize = 11.sp,
                            color = TerminalAccent
                        )
                    }
                }
            }
        }
    }
}

