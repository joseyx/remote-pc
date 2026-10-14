package com.remotepc.app.ui.monitor

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.remotepc.app.data.model.ProcessInfo
import com.remotepc.app.data.model.ServerInfoResponse
import com.remotepc.app.data.model.SystemStats
import com.remotepc.app.data.model.VpnStatus
import com.remotepc.app.data.network.ApiClient
import com.remotepc.app.ui.theme.*
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

@Composable
fun MonitorScreen(
    apiClient: ApiClient
) {
    val scope = rememberCoroutineScope()
    var stats by remember { mutableStateOf<SystemStats?>(null) }
    var serverInfo by remember { mutableStateOf<ServerInfoResponse?>(null) }
    var vpnStatus by remember { mutableStateOf<VpnStatus?>(null) }
    var isVpnLoading by remember { mutableStateOf(false) }
    var isLoading by remember { mutableStateOf(false) }
    var processToKill by remember { mutableStateOf<ProcessInfo?>(null) }

    fun refreshMonitor() {
        scope.launch {
            isLoading = true
            try {
                stats = apiClient.getSystemStats(withTop = true)
                serverInfo = apiClient.getServerInfo()
                vpnStatus = apiClient.getVpnStatus()
            } catch (e: Exception) {
                // handle error
            } finally {
                isLoading = false
            }
        }
    }

    LaunchedEffect(Unit) {
        refreshMonitor()
    }

    fun connectVpn(location: String = "", protocol: String = "") {
        scope.launch {
            isVpnLoading = true
            try {
                apiClient.connectVpn(location = location, protocol = protocol)
                for (i in 1..5) {
                    delay(2000)
                    val updated = apiClient.getVpnStatus()
                    vpnStatus = updated
                    if (updated.connected) break
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
                for (i in 1..5) {
                    delay(2000)
                    val updated = apiClient.getVpnStatus()
                    vpnStatus = updated
                    if (!updated.connected) break
                }
            } catch (e: Exception) {
                // handle error
            } finally {
                isVpnLoading = false
            }
        }
    }

    LazyColumn(
        modifier = Modifier
            .fillMaxSize()
            .padding(horizontal = 14.dp, vertical = 6.dp),
        verticalArrangement = Arrangement.spacedBy(14.dp),
        contentPadding = PaddingValues(bottom = 80.dp)
    ) {
        // Section 1: Host Overview Card
        item {
            Card(
                shape = RoundedCornerShape(14.dp),
                colors = CardDefaults.cardColors(containerColor = TerminalSurface),
                border = BorderStroke(1.dp, TerminalBorder),
                modifier = Modifier.fillMaxWidth()
            ) {
                Column(modifier = Modifier.padding(14.dp)) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.SpaceBetween
                    ) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Icon(Icons.Default.Computer, contentDescription = null, tint = TerminalAccent, modifier = Modifier.size(18.dp))
                            Spacer(modifier = Modifier.width(8.dp))
                            Text(
                                text = serverInfo?.hostname?.ifBlank { "Personal PC" } ?: "Personal PC",
                                color = TerminalText,
                                fontWeight = FontWeight.Bold,
                                fontSize = 15.sp
                            )
                        }
                        Surface(
                            color = TerminalAccent.copy(alpha = 0.15f),
                            shape = RoundedCornerShape(4.dp)
                        ) {
                            Text(
                                text = "v${serverInfo?.version?.ifBlank { "1.2.1" } ?: "1.2.1"}",
                                color = TerminalAccent,
                                fontSize = 10.sp,
                                fontWeight = FontWeight.Bold,
                                modifier = Modifier.padding(horizontal = 5.dp, vertical = 2.dp)
                            )
                        }
                    }

                    Spacer(modifier = Modifier.height(10.dp))

                    val lanIps = serverInfo?.lanIps?.joinToString(", ") ?: "127.0.0.1"
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Icon(Icons.Default.Lan, contentDescription = null, tint = TerminalTextDim, modifier = Modifier.size(14.dp))
                        Spacer(modifier = Modifier.width(6.dp))
                        Text("LAN IP: $lanIps", color = TerminalTextMuted, fontSize = 12.sp, fontFamily = FontFamily.Monospace)
                    }

                    if (!serverInfo?.uptime.isNullOrBlank()) {
                        Spacer(modifier = Modifier.height(4.dp))
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Icon(Icons.Default.Schedule, contentDescription = null, tint = TerminalTextDim, modifier = Modifier.size(14.dp))
                            Spacer(modifier = Modifier.width(6.dp))
                            Text("Uptime: ${serverInfo?.uptime}", color = TerminalTextDim, fontSize = 11.sp)
                        }
                    }
                }
            }
        }

        // Section 2: Memory & Resources
        if (stats != null) {
            item {
                Card(
                    shape = RoundedCornerShape(14.dp),
                    colors = CardDefaults.cardColors(containerColor = TerminalSurface),
                    border = BorderStroke(1.dp, TerminalBorder),
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Column(modifier = Modifier.padding(14.dp)) {
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.SpaceBetween
                        ) {
                            Row(verticalAlignment = Alignment.CenterVertically) {
                                Icon(Icons.Default.Memory, contentDescription = null, tint = TerminalGreen, modifier = Modifier.size(18.dp))
                                Spacer(modifier = Modifier.width(8.dp))
                                Text("RAM Memory Usage", color = TerminalText, fontWeight = FontWeight.Bold, fontSize = 14.sp)
                            }
                            Text(
                                text = "${stats!!.memory.percent}%",
                                color = if (stats!!.memory.percent > 85) TerminalRed else TerminalGreen,
                                fontWeight = FontWeight.ExtraBold,
                                fontSize = 14.sp
                            )
                        }

                        Spacer(modifier = Modifier.height(10.dp))

                        // Progress Bar
                        LinearProgressIndicator(
                            progress = { stats!!.memory.percent / 100f },
                            modifier = Modifier
                                .fillMaxWidth()
                                .height(8.dp)
                                .clip(RoundedCornerShape(4.dp)),
                            color = if (stats!!.memory.percent > 85) TerminalRed else TerminalGreen,
                            trackColor = TerminalSurfaceElevated
                        )

                        Spacer(modifier = Modifier.height(8.dp))

                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.SpaceBetween
                        ) {
                            Text(
                                text = "Used: ${String.format("%.1f", stats!!.memory.usedGB)} GB",
                                color = TerminalTextMuted,
                                fontSize = 11.sp
                            )
                            Text(
                                text = "Total: ${String.format("%.1f", stats!!.memory.totalGB)} GB",
                                color = TerminalTextDim,
                                fontSize = 11.sp
                            )
                        }

                        // Monitored Services Chips
                        if (stats!!.services.isNotEmpty()) {
                            Spacer(modifier = Modifier.height(12.dp))
                            HorizontalDivider(color = TerminalDivider)
                            Spacer(modifier = Modifier.height(10.dp))
                            Text("Local Services", color = TerminalTextDim, fontSize = 11.sp, fontWeight = FontWeight.Bold)
                            Spacer(modifier = Modifier.height(6.dp))
                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                horizontalArrangement = Arrangement.spacedBy(8.dp)
                            ) {
                                stats!!.services.forEach { s ->
                                    Surface(
                                        color = if (s.running) TerminalGreen.copy(alpha = 0.15f) else TerminalTextDim.copy(alpha = 0.15f),
                                        shape = RoundedCornerShape(6.dp)
                                    ) {
                                        Row(
                                            verticalAlignment = Alignment.CenterVertically,
                                            modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp)
                                        ) {
                                            Box(
                                                modifier = Modifier
                                                    .size(6.dp)
                                                    .clip(CircleShape)
                                                    .background(if (s.running) TerminalGreen else TerminalTextDim)
                                            )
                                            Spacer(modifier = Modifier.width(6.dp))
                                            Text(
                                                text = "${s.name} (:${s.port})",
                                                color = if (s.running) TerminalGreen else TerminalTextDim,
                                                fontSize = 11.sp,
                                                fontWeight = FontWeight.SemiBold
                                            )
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }

        // Section 3: VPN Guard
        if (vpnStatus != null && vpnStatus!!.installed) {
            item {
                Card(
                    shape = RoundedCornerShape(14.dp),
                    colors = CardDefaults.cardColors(containerColor = TerminalSurface),
                    border = BorderStroke(1.dp, if (vpnStatus!!.connected) TerminalAccent.copy(alpha = 0.4f) else TerminalBorder),
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Column(modifier = Modifier.padding(14.dp)) {
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.SpaceBetween
                        ) {
                            Row(verticalAlignment = Alignment.CenterVertically) {
                                Icon(Icons.Default.Security, contentDescription = null, tint = if (vpnStatus!!.connected) TerminalAccent else TerminalTextDim, modifier = Modifier.size(18.dp))
                                Spacer(modifier = Modifier.width(8.dp))
                                Text("Windscribe VPN Guard", color = TerminalText, fontWeight = FontWeight.Bold, fontSize = 14.sp)
                            }

                            Surface(
                                color = if (vpnStatus!!.connected) TerminalGreen.copy(alpha = 0.15f) else TerminalTextDim.copy(alpha = 0.15f),
                                shape = RoundedCornerShape(4.dp)
                            ) {
                                Text(
                                    text = if (vpnStatus!!.connected) "PROTECTED" else "DISCONNECTED",
                                    color = if (vpnStatus!!.connected) TerminalGreen else TerminalTextDim,
                                    fontSize = 10.sp,
                                    fontWeight = FontWeight.Bold,
                                    modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp)
                                )
                            }
                        }

                        if (vpnStatus!!.connected) {
                            Spacer(modifier = Modifier.height(8.dp))
                            Text("Location: ${vpnStatus!!.location} • IP: ${vpnStatus!!.ip}", color = TerminalTextMuted, fontSize = 11.sp, fontFamily = FontFamily.Monospace)
                        }

                        Spacer(modifier = Modifier.height(12.dp))

                        if (vpnStatus!!.connected) {
                            Button(
                                onClick = { disconnectVpn() },
                                colors = ButtonDefaults.buttonColors(containerColor = TerminalRed.copy(alpha = 0.2f), contentColor = TerminalRedBright),
                                shape = RoundedCornerShape(8.dp),
                                modifier = Modifier.fillMaxWidth()
                            ) {
                                if (isVpnLoading) {
                                    CircularProgressIndicator(color = TerminalRedBright, modifier = Modifier.size(16.dp))
                                } else {
                                    Icon(Icons.Default.PowerSettingsNew, contentDescription = null, modifier = Modifier.size(16.dp))
                                    Spacer(modifier = Modifier.width(6.dp))
                                    Text("Disconnect VPN", fontSize = 12.sp)
                                }
                            }
                        } else {
                            Button(
                                onClick = { connectVpn("best", "wireguard") },
                                colors = ButtonDefaults.buttonColors(containerColor = TerminalAccent),
                                shape = RoundedCornerShape(8.dp),
                                modifier = Modifier.fillMaxWidth()
                            ) {
                                if (isVpnLoading) {
                                    CircularProgressIndicator(color = TerminalBackground, modifier = Modifier.size(16.dp))
                                } else {
                                    Icon(Icons.Default.Lock, contentDescription = null, tint = TerminalBackground, modifier = Modifier.size(16.dp))
                                    Spacer(modifier = Modifier.width(6.dp))
                                    Text("Connect (Best Location)", color = TerminalBackground, fontSize = 12.sp, fontWeight = FontWeight.Bold)
                                }
                            }
                        }
                    }
                }
            }
        }

        // Section 4: Top Processes
        val topProcs = stats?.topProcesses ?: emptyList()
        item {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Icon(Icons.Default.FormatListNumbered, contentDescription = null, tint = TerminalAccent, modifier = Modifier.size(16.dp))
                Spacer(modifier = Modifier.width(6.dp))
                Text("Top Consuming Processes", color = TerminalText, fontWeight = FontWeight.Bold, fontSize = 14.sp)
            }
        }

        if (topProcs.isNotEmpty()) {
            items(topProcs) { proc ->
                Surface(
                    color = TerminalSurface,
                    shape = RoundedCornerShape(10.dp),
                    border = BorderStroke(1.dp, TerminalBorder),
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 12.dp, vertical = 8.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.SpaceBetween
                    ) {
                        Column {
                            Text(proc.name, color = TerminalText, fontWeight = FontWeight.Bold, fontSize = 13.sp)
                            Text("PID: ${proc.pid} • RAM: ${String.format("%.0f", proc.ramMB)} MB", color = TerminalTextDim, fontSize = 11.sp, fontFamily = FontFamily.Monospace)
                        }

                        IconButton(
                            onClick = { processToKill = proc },
                            modifier = Modifier.size(28.dp)
                        ) {
                            Icon(Icons.Default.Cancel, contentDescription = "Kill", tint = TerminalRed.copy(alpha = 0.7f), modifier = Modifier.size(18.dp))
                        }
                    }
                }
            }
        }
    }

    // Kill confirmation dialog
    if (processToKill != null) {
        AlertDialog(
            onDismissRequest = { processToKill = null },
            containerColor = TerminalSurface,
            title = {
                Text("Kill Process?", color = TerminalRedBright, style = MaterialTheme.typography.titleMedium)
            },
            text = {
                Text(
                    text = "Are you sure you want to terminate '${processToKill!!.name}' (PID: ${processToKill!!.pid})? Unsaved work in this process will be lost.",
                    color = TerminalTextMuted,
                    fontSize = 13.sp
                )
            },
            confirmButton = {
                Button(
                    onClick = {
                        val p = processToKill
                        processToKill = null
                        if (p != null) {
                            scope.launch {
                                apiClient.killProcess(p.pid)
                                refreshMonitor()
                            }
                        }
                    },
                    colors = ButtonDefaults.buttonColors(containerColor = TerminalRed)
                ) {
                    Text("Kill Process", color = Color.White)
                }
            },
            dismissButton = {
                TextButton(onClick = { processToKill = null }) {
                    Text("Cancel", color = TerminalTextMuted)
                }
            }
        )
    }
}
