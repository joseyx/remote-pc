package com.remotepc.app.ui.bubbles

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
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.remotepc.app.data.model.Bubble
import com.remotepc.app.data.model.CreateSessionRequest
import com.remotepc.app.data.network.ApiClient
import com.remotepc.app.ui.dialogs.CreateBubbleDialog
import com.remotepc.app.ui.theme.*
import kotlinx.coroutines.launch

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun BubblesScreen(
    apiClient: ApiClient,
    onOpenSession: (sessionId: String, sessionName: String) -> Unit
) {
    val scope = rememberCoroutineScope()
    var bubbles by remember { mutableStateOf<List<Bubble>>(emptyList()) }
    var isLoading by remember { mutableStateOf(false) }
    var selectedWorkspace by remember { mutableStateOf("all") }
    var searchQuery by remember { mutableStateOf("") }
    var showCreateDialog by remember { mutableStateOf(false) }
    var actionMessage by remember { mutableStateOf<String?>(null) }

    fun refreshBubbles() {
        scope.launch {
            isLoading = true
            try {
                bubbles = apiClient.listBubbles(scan = true)
            } catch (e: Exception) {
                // handle error
            } finally {
                isLoading = false
            }
        }
    }

    LaunchedEffect(Unit) {
        refreshBubbles()
    }

    val workspaces = remember(bubbles) {
        val set = mutableSetOf("all")
        bubbles.forEach { if (it.workspace.isNotBlank()) set.add(it.workspace.lowercase()) }
        set.toList()
    }

    val filteredBubbles = remember(bubbles, selectedWorkspace, searchQuery) {
        bubbles.filter { b ->
            val matchWs = selectedWorkspace == "all" || b.workspace.equals(selectedWorkspace, ignoreCase = true)
            val matchQuery = searchQuery.isBlank() ||
                    b.name.contains(searchQuery, ignoreCase = true) ||
                    b.branch.contains(searchQuery, ignoreCase = true) ||
                    b.id.contains(searchQuery, ignoreCase = true)
            matchWs && matchQuery
        }
    }

    Box(modifier = Modifier.fillMaxSize()) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(horizontal = 14.dp, vertical = 6.dp)
        ) {
            // Workspace selector horizontal chips
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .horizontalScroll(rememberScrollState()),
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                workspaces.forEach { ws ->
                    val isSelected = selectedWorkspace == ws
                    val label = if (ws == "all") "All Workspaces" else ws.replaceFirstChar { it.uppercase() }
                    FilterChip(
                        selected = isSelected,
                        onClick = { selectedWorkspace = ws },
                        label = {
                            Text(
                                text = label,
                                fontWeight = if (isSelected) FontWeight.Bold else FontWeight.Normal,
                                fontSize = 12.sp
                            )
                        },
                        colors = FilterChipDefaults.filterChipColors(
                            selectedContainerColor = TerminalAccent.copy(alpha = 0.2f),
                            selectedLabelColor = TerminalAccent,
                            containerColor = TerminalSurface,
                            labelColor = TerminalTextMuted
                        ),
                        border = FilterChipDefaults.filterChipBorder(
                            enabled = true,
                            selected = isSelected,
                            borderColor = if (isSelected) TerminalAccent else TerminalBorder
                        )
                    )
                }
            }

            Spacer(modifier = Modifier.height(10.dp))

            // Search Bar
            OutlinedTextField(
                value = searchQuery,
                onValueChange = { searchQuery = it },
                placeholder = { Text("Filter tasks (e.g. TAR-1125, hotfix...)", color = TerminalTextDim, fontSize = 13.sp) },
                leadingIcon = { Icon(Icons.Default.Search, contentDescription = null, tint = TerminalTextDim, modifier = Modifier.size(18.dp)) },
                trailingIcon = {
                    if (searchQuery.isNotEmpty()) {
                        IconButton(onClick = { searchQuery = "" }) {
                            Icon(Icons.Default.Close, contentDescription = "Clear", tint = TerminalTextDim, modifier = Modifier.size(16.dp))
                        }
                    }
                },
                singleLine = true,
                shape = RoundedCornerShape(12.dp),
                colors = OutlinedTextFieldDefaults.colors(
                    focusedTextColor = TerminalText,
                    unfocusedTextColor = TerminalText,
                    focusedContainerColor = TerminalSurface,
                    unfocusedContainerColor = TerminalSurface,
                    focusedBorderColor = TerminalAccent,
                    unfocusedBorderColor = TerminalBorder
                ),
                modifier = Modifier
                    .fillMaxWidth()
                    .height(50.dp)
            )

            Spacer(modifier = Modifier.height(12.dp))

            when {
                isLoading && bubbles.isEmpty() -> {
                    Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                        CircularProgressIndicator(color = TerminalAccent)
                    }
                }
                filteredBubbles.isEmpty() -> {
                    Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                        Column(horizontalAlignment = Alignment.CenterHorizontally) {
                            Icon(Icons.Default.FolderOpen, contentDescription = null, tint = TerminalTextDim, modifier = Modifier.size(48.dp))
                            Spacer(modifier = Modifier.height(8.dp))
                            Text("No bubbles found", color = TerminalTextMuted, fontSize = 14.sp)
                            Spacer(modifier = Modifier.height(4.dp))
                            Text("Tap the + button to create a new task worktree", color = TerminalTextDim, fontSize = 12.sp)
                        }
                    }
                }
                else -> {
                    LazyColumn(
                        modifier = Modifier.fillMaxSize(),
                        verticalArrangement = Arrangement.spacedBy(10.dp),
                        contentPadding = PaddingValues(bottom = 80.dp)
                    ) {
                        items(filteredBubbles, key = { it.id }) { bubble ->
                            BubbleCard(
                                bubble = bubble,
                                onStartDev = {
                                    scope.launch {
                                        apiClient.startDevServer(bubble.id)
                                        refreshBubbles()
                                    }
                                },
                                onStopDev = {
                                    scope.launch {
                                        apiClient.stopDevServer(bubble.id)
                                        refreshBubbles()
                                    }
                                },
                                onLaunchClaude = {
                                    scope.launch {
                                        try {
                                            val created = apiClient.createSession(
                                                CreateSessionRequest(
                                                    name = "🤖 Claude (${bubble.name})",
                                                    cwd = bubble.path,
                                                    command = "claude",
                                                    bubbleId = bubble.id,
                                                    role = "claude"
                                                )
                                            )
                                            onOpenSession(created.id, created.name)
                                        } catch (e: Exception) {
                                            actionMessage = "Error launching Claude: ${e.message}"
                                        }
                                    }
                                },
                                onLaunchShell = {
                                    scope.launch {
                                        try {
                                            val created = apiClient.createSession(
                                                CreateSessionRequest(
                                                    name = "🐚 ${bubble.name}",
                                                    cwd = bubble.path,
                                                    command = "powershell.exe",
                                                    bubbleId = bubble.id,
                                                    role = "shell"
                                                )
                                            )
                                            onOpenSession(created.id, created.name)
                                        } catch (e: Exception) {
                                            actionMessage = "Error opening shell: ${e.message}"
                                        }
                                    }
                                },
                                onDelete = {
                                    scope.launch {
                                        apiClient.deleteBubble(bubble.id)
                                        refreshBubbles()
                                    }
                                }
                            )
                        }
                    }
                }
            }
        }

        // Floating Action Button to create a new bubble
        FloatingActionButton(
            onClick = { showCreateDialog = true },
            containerColor = TerminalAccent,
            contentColor = TerminalBackground,
            modifier = Modifier
                .align(Alignment.BottomEnd)
                .padding(20.dp)
        ) {
            Icon(Icons.Default.Add, contentDescription = "New Bubble")
        }
    }

    if (showCreateDialog) {
        CreateBubbleDialog(
            apiClient = apiClient,
            onDismiss = { showCreateDialog = false },
            onCreated = { req ->
                showCreateDialog = false
                scope.launch {
                    try {
                        apiClient.createBubble(req)
                        refreshBubbles()
                    } catch (e: Exception) {
                        actionMessage = "Error creating bubble: ${e.message}"
                    }
                }
            }
        )
    }
}

@Composable
private fun BubbleCard(
    bubble: Bubble,
    onStartDev: () -> Unit,
    onStopDev: () -> Unit,
    onLaunchClaude: () -> Unit,
    onLaunchShell: () -> Unit,
    onDelete: () -> Unit
) {
    Card(
        shape = RoundedCornerShape(14.dp),
        colors = CardDefaults.cardColors(containerColor = TerminalSurface),
        border = BorderStroke(1.dp, if (bubble.devRunning) TerminalGreen.copy(alpha = 0.4f) else TerminalBorder),
        modifier = Modifier.fillMaxWidth()
    ) {
        Column(modifier = Modifier.padding(14.dp)) {
            // Top Row: Workspace Badge, Title, Priority Pill, Menu/Delete
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    val wsColor = when (bubble.workspace.lowercase()) {
                        "aditum" -> TerminalAccent
                        "brif" -> TerminalPurple
                        else -> TerminalTextMuted
                    }
                    Surface(
                        color = wsColor.copy(alpha = 0.15f),
                        shape = RoundedCornerShape(6.dp)
                    ) {
                        Text(
                            text = bubble.workspace.uppercase(),
                            color = wsColor,
                            fontSize = 10.sp,
                            fontWeight = FontWeight.ExtraBold,
                            modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp)
                        )
                    }

                    Spacer(modifier = Modifier.width(8.dp))

                    Text(
                        text = bubble.name,
                        color = TerminalText,
                        fontSize = 16.sp,
                        fontWeight = FontWeight.Bold
                    )

                    if (bubble.priority.isNotBlank()) {
                        Spacer(modifier = Modifier.width(6.dp))
                        val pColor = when (bubble.priority.uppercase()) {
                            "P0" -> TerminalRed
                            "P1" -> TerminalAmber
                            else -> TerminalAccent
                        }
                        Surface(
                            color = pColor.copy(alpha = 0.15f),
                            shape = RoundedCornerShape(4.dp)
                        ) {
                            Text(
                                text = bubble.priority.uppercase(),
                                color = pColor,
                                fontSize = 10.sp,
                                fontWeight = FontWeight.Bold,
                                modifier = Modifier.padding(horizontal = 5.dp, vertical = 1.dp)
                            )
                        }
                    }
                }

                IconButton(onClick = onDelete, modifier = Modifier.size(24.dp)) {
                    Icon(Icons.Default.DeleteOutline, contentDescription = "Delete", tint = TerminalTextDim)
                }
            }

            Spacer(modifier = Modifier.height(8.dp))

            // Branch & Path
            Row(verticalAlignment = Alignment.CenterVertically) {
                Icon(
                    imageVector = Icons.Default.Commit,
                    contentDescription = null,
                    tint = TerminalTextMuted,
                    modifier = Modifier.size(14.dp)
                )
                Spacer(modifier = Modifier.width(5.dp))
                Text(
                    text = bubble.branch.ifBlank { "main" },
                    color = TerminalTextMuted,
                    fontFamily = FontFamily.Monospace,
                    fontSize = 12.sp
                )
                if (bubble.port > 0) {
                    Text(
                        text = " • :${bubble.port}",
                        color = TerminalAccent,
                        fontSize = 12.sp,
                        fontFamily = FontFamily.Monospace
                    )
                }
            }

            if (bubble.dbSchema.isNotBlank()) {
                Spacer(modifier = Modifier.height(2.dp))
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Icon(
                        imageVector = Icons.Default.Storage,
                        contentDescription = null,
                        tint = TerminalTextDim,
                        modifier = Modifier.size(13.dp)
                    )
                    Spacer(modifier = Modifier.width(5.dp))
                    Text(
                        text = "DB: ${bubble.dbSchema}",
                        color = TerminalTextDim,
                        fontFamily = FontFamily.Monospace,
                        fontSize = 11.sp
                    )
                }
            }

            Spacer(modifier = Modifier.height(10.dp))
            HorizontalDivider(color = TerminalDivider)
            Spacer(modifier = Modifier.height(10.dp))

            // Action Row: Dev Server Button + Quick Launch Claude & Terminal
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                // Dev Server Status / Action
                if (bubble.devRunning) {
                    Surface(
                        color = TerminalGreen.copy(alpha = 0.15f),
                        shape = RoundedCornerShape(8.dp),
                        modifier = Modifier.clickable { onStopDev() }
                    ) {
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            modifier = Modifier.padding(horizontal = 10.dp, vertical = 6.dp)
                        ) {
                            Box(
                                modifier = Modifier
                                    .size(8.dp)
                                    .clip(CircleShape)
                                    .background(TerminalGreen)
                            )
                            Spacer(modifier = Modifier.width(6.dp))
                            Text("RUNNING", color = TerminalGreen, fontSize = 11.sp, fontWeight = FontWeight.Bold)
                            Spacer(modifier = Modifier.width(6.dp))
                            Icon(Icons.Default.Stop, contentDescription = "Stop", tint = TerminalGreen, modifier = Modifier.size(14.dp))
                        }
                    }
                } else {
                    OutlinedButton(
                        onClick = onStartDev,
                        shape = RoundedCornerShape(8.dp),
                        contentPadding = PaddingValues(horizontal = 10.dp, vertical = 4.dp),
                        border = BorderStroke(1.dp, TerminalBorder)
                    ) {
                        Icon(Icons.Default.PlayArrow, contentDescription = null, tint = TerminalAccent, modifier = Modifier.size(14.dp))
                        Spacer(modifier = Modifier.width(4.dp))
                        Text("Start Server", color = TerminalText, fontSize = 11.sp)
                    }
                }

                // Quick Launchers
                Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    FilledTonalButton(
                        onClick = onLaunchClaude,
                        shape = RoundedCornerShape(8.dp),
                        contentPadding = PaddingValues(horizontal = 10.dp, vertical = 4.dp),
                        colors = ButtonDefaults.filledTonalButtonColors(containerColor = TerminalPurple.copy(alpha = 0.2f))
                    ) {
                        Text("🤖 Claude", color = TerminalPurpleBright, fontSize = 11.sp, fontWeight = FontWeight.SemiBold)
                    }

                    FilledTonalButton(
                        onClick = onLaunchShell,
                        shape = RoundedCornerShape(8.dp),
                        contentPadding = PaddingValues(horizontal = 8.dp, vertical = 4.dp),
                        colors = ButtonDefaults.filledTonalButtonColors(containerColor = TerminalSurfaceElevated)
                    ) {
                        Icon(Icons.Default.Terminal, contentDescription = "Terminal", tint = TerminalTextMuted, modifier = Modifier.size(14.dp))
                    }
                }
            }
        }
    }
}
