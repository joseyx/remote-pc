package com.remotepc.app.ui.sessions

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
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
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.remotepc.app.data.model.Session
import com.remotepc.app.data.network.ApiClient
import com.remotepc.app.ui.dialogs.CreateSessionDialog
import com.remotepc.app.ui.theme.*
import kotlinx.coroutines.launch

@Composable
fun SessionListScreen(
    apiClient: ApiClient,
    onOpenSession: (Session) -> Unit
) {
    val scope = rememberCoroutineScope()
    var sessions by remember { mutableStateOf<List<Session>>(emptyList()) }
    var isLoading by remember { mutableStateOf(false) }
    var selectedFilter by remember { mutableStateOf("all") }
    var showCreateDialog by remember { mutableStateOf(false) }
    var errorMessage by remember { mutableStateOf<String?>(null) }

    fun refreshSessions() {
        scope.launch {
            isLoading = true
            errorMessage = null
            try {
                sessions = apiClient.listSessions()
            } catch (e: Exception) {
                errorMessage = "Failed to load sessions: ${e.message}"
            } finally {
                isLoading = false
            }
        }
    }

    LaunchedEffect(Unit) {
        refreshSessions()
    }

    val filteredSessions = remember(sessions, selectedFilter) {
        when (selectedFilter) {
            "server" -> sessions.filter { it.role == "server" }
            "agent" -> sessions.filter { it.role == "claude" || it.role == "agy" }
            "shell" -> sessions.filter { it.role == "shell" || it.role.isBlank() }
            else -> sessions
        }
    }

    Box(modifier = Modifier.fillMaxSize()) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(horizontal = 14.dp, vertical = 6.dp)
        ) {
            // Category Filter Chips
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .horizontalScroll(rememberScrollState()),
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                val serverCount = sessions.count { it.role == "server" }
                val agentCount = sessions.count { it.role == "claude" || it.role == "agy" }
                val shellCount = sessions.count { it.role == "shell" || it.role.isBlank() }

                FilterChip(
                    selected = selectedFilter == "all",
                    onClick = { selectedFilter = "all" },
                    label = { Text("All (${sessions.size})", fontSize = 12.sp) },
                    colors = FilterChipDefaults.filterChipColors(
                        selectedContainerColor = TerminalAccent.copy(alpha = 0.2f),
                        selectedLabelColor = TerminalAccent,
                        containerColor = TerminalSurface,
                        labelColor = TerminalTextMuted
                    )
                )

                FilterChip(
                    selected = selectedFilter == "server",
                    onClick = { selectedFilter = "server" },
                    label = { Text("⚙️ Servers ($serverCount)", fontSize = 12.sp) },
                    colors = FilterChipDefaults.filterChipColors(
                        selectedContainerColor = TerminalAmber.copy(alpha = 0.2f),
                        selectedLabelColor = TerminalAmberBright,
                        containerColor = TerminalSurface,
                        labelColor = TerminalTextMuted
                    )
                )

                FilterChip(
                    selected = selectedFilter == "agent",
                    onClick = { selectedFilter = "agent" },
                    label = { Text("🤖 Agents ($agentCount)", fontSize = 12.sp) },
                    colors = FilterChipDefaults.filterChipColors(
                        selectedContainerColor = TerminalPurple.copy(alpha = 0.2f),
                        selectedLabelColor = TerminalPurpleBright,
                        containerColor = TerminalSurface,
                        labelColor = TerminalTextMuted
                    )
                )

                FilterChip(
                    selected = selectedFilter == "shell",
                    onClick = { selectedFilter = "shell" },
                    label = { Text("🐚 Shells ($shellCount)", fontSize = 12.sp) },
                    colors = FilterChipDefaults.filterChipColors(
                        selectedContainerColor = TerminalAccent.copy(alpha = 0.2f),
                        selectedLabelColor = TerminalAccent,
                        containerColor = TerminalSurface,
                        labelColor = TerminalTextMuted
                    )
                )
            }

            Spacer(modifier = Modifier.height(12.dp))

            when {
                isLoading && sessions.isEmpty() -> {
                    Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                        CircularProgressIndicator(color = TerminalAccent)
                    }
                }
                filteredSessions.isEmpty() -> {
                    Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                        Column(
                            horizontalAlignment = Alignment.CenterHorizontally,
                            modifier = Modifier.padding(24.dp)
                        ) {
                            Icon(
                                imageVector = Icons.Default.Terminal,
                                contentDescription = null,
                                tint = TerminalTextDim,
                                modifier = Modifier.size(52.dp)
                            )
                            Spacer(modifier = Modifier.height(10.dp))
                            Text(
                                text = "No active terminals",
                                color = TerminalText,
                                style = MaterialTheme.typography.titleMedium
                            )
                            Spacer(modifier = Modifier.height(6.dp))
                            Text(
                                text = "Tap the + button below to start a dev server, launch Claude Code, or open a shell.",
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
                        verticalArrangement = Arrangement.spacedBy(10.dp),
                        contentPadding = PaddingValues(bottom = 80.dp)
                    ) {
                        items(filteredSessions, key = { it.id }) { session ->
                            ModernSessionCard(
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

        // Floating Action Button
        FloatingActionButton(
            onClick = { showCreateDialog = true },
            containerColor = TerminalAccent,
            contentColor = TerminalBackground,
            modifier = Modifier
                .align(Alignment.BottomEnd)
                .padding(20.dp)
        ) {
            Icon(Icons.Default.Add, contentDescription = "New Session")
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
                        errorMessage = "Error creating session: ${e.message}"
                    }
                }
            }
        )
    }
}

@Composable
private fun ModernSessionCard(
    session: Session,
    onClick: () -> Unit,
    onDelete: () -> Unit
) {
    val (roleIcon, roleColor, roleLabel) = when (session.role) {
        "server" -> Triple("⚙️", TerminalAmber, "DEV SERVER")
        "claude" -> Triple("🤖", TerminalPurple, "CLAUDE CODE")
        "agy" -> Triple("🚀", TerminalAccent, "ANTIGRAVITY")
        else -> Triple("🐚", TerminalTextMuted, "SHELL")
    }

    Card(
        modifier = Modifier
            .fillMaxWidth()
            .clickable { onClick() },
        shape = RoundedCornerShape(14.dp),
        colors = CardDefaults.cardColors(containerColor = TerminalSurface),
        border = BorderStroke(1.dp, if (session.role == "server") TerminalAmber.copy(alpha = 0.4f) else TerminalBorder)
    ) {
        Column(modifier = Modifier.padding(14.dp)) {
            // Header: Role icon, Name, Status Pill, Close Button
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.weight(1f)) {
                    Text(text = roleIcon, fontSize = 18.sp)
                    Spacer(modifier = Modifier.width(8.dp))
                    Column {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(
                                text = session.name,
                                style = MaterialTheme.typography.titleMedium,
                                fontWeight = FontWeight.Bold,
                                color = TerminalText
                            )
                        }
                        Text(
                            text = roleLabel,
                            color = roleColor,
                            fontSize = 10.sp,
                            fontWeight = FontWeight.ExtraBold
                        )
                    }
                }

                Row(verticalAlignment = Alignment.CenterVertically) {
                    Surface(
                        color = if (session.isRunning) TerminalGreen.copy(alpha = 0.15f) else TerminalRed.copy(alpha = 0.15f),
                        shape = RoundedCornerShape(6.dp)
                    ) {
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            modifier = Modifier.padding(horizontal = 7.dp, vertical = 3.dp)
                        ) {
                            Box(
                                modifier = Modifier
                                    .size(6.dp)
                                    .clip(CircleShape)
                                    .background(if (session.isRunning) TerminalGreen else TerminalRed)
                            )
                            Spacer(modifier = Modifier.width(4.dp))
                            Text(
                                text = if (session.isRunning) "PID ${session.pid}" else "STOPPED",
                                fontSize = 10.sp,
                                fontWeight = FontWeight.Bold,
                                color = if (session.isRunning) TerminalGreen else TerminalRed
                            )
                        }
                    }

                    Spacer(modifier = Modifier.width(6.dp))

                    IconButton(onClick = onDelete, modifier = Modifier.size(26.dp)) {
                        Icon(Icons.Default.Close, contentDescription = "Terminate", tint = TerminalTextDim, modifier = Modifier.size(16.dp))
                    }
                }
            }

            Spacer(modifier = Modifier.height(10.dp))

            // Working Directory
            Row(verticalAlignment = Alignment.CenterVertically) {
                Icon(
                    imageVector = Icons.Default.Folder,
                    contentDescription = null,
                    tint = TerminalTextMuted,
                    modifier = Modifier.size(14.dp)
                )
                Spacer(modifier = Modifier.width(6.dp))
                Text(
                    text = session.cwd.ifEmpty { "Default Workspace" },
                    fontSize = 11.sp,
                    fontFamily = FontFamily.Monospace,
                    color = TerminalTextMuted
                )
            }

            Spacer(modifier = Modifier.height(6.dp))

            // Command Snippet
            Surface(
                color = TerminalSurfaceElevated,
                shape = RoundedCornerShape(6.dp),
                modifier = Modifier.fillMaxWidth()
            ) {
                Text(
                    text = "> ${session.command}",
                    fontFamily = FontFamily.Monospace,
                    fontSize = 11.sp,
                    color = TerminalAccent,
                    modifier = Modifier.padding(horizontal = 8.dp, vertical = 5.dp)
                )
            }
        }
    }
}
