package com.remotepc.app.ui.dialogs

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Folder
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.remotepc.app.data.model.CreateSessionRequest
import com.remotepc.app.data.model.DirItem
import com.remotepc.app.data.network.ApiClient
import com.remotepc.app.ui.theme.*
import kotlinx.coroutines.launch

@Composable
fun CreateSessionDialog(
    apiClient: ApiClient,
    initialCwd: String = "",
    initialBubbleId: String = "",
    onDismiss: () -> Unit,
    onCreated: (CreateSessionRequest) -> Unit
) {
    var name by remember { mutableStateOf("") }
    var cwd by remember { mutableStateOf(initialCwd) }
    var bubbleId by remember { mutableStateOf(initialBubbleId) }
    var selectedPreset by remember { mutableStateOf("claude") }
    var customCommand by remember { mutableStateOf("") }

    var browsingDirs by remember { mutableStateOf(false) }
    var currentDirPath by remember { mutableStateOf(initialCwd) }
    var parentDirPath by remember { mutableStateOf("") }
    var dirList by remember { mutableStateOf<List<DirItem>>(emptyList()) }

    val scope = rememberCoroutineScope()

    fun loadDir(path: String) {
        scope.launch {
            try {
                val res = apiClient.listDirectories(path)
                currentDirPath = res.current
                parentDirPath = res.parent
                dirList = res.dirs
                cwd = res.current
            } catch (e: Exception) {
                // Keep manual input
            }
        }
    }

    LaunchedEffect(browsingDirs) {
        if (browsingDirs && dirList.isEmpty()) {
            loadDir(currentDirPath)
        }
    }

    AlertDialog(
        onDismissRequest = onDismiss,
        containerColor = TerminalSurface,
        title = {
            Text(
                text = if (browsingDirs) "Select Folder on PC" else "New Terminal Session",
                color = TerminalText,
                style = MaterialTheme.typography.titleMedium
            )
        },
        text = {
            if (browsingDirs) {
                Column(modifier = Modifier.fillMaxWidth().heightIn(max = 350.dp)) {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        modifier = Modifier.fillMaxWidth().padding(bottom = 8.dp)
                    ) {
                        IconButton(onClick = { loadDir(parentDirPath) }) {
                            Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Up", tint = TerminalAccent)
                        }
                        Text(
                            text = currentDirPath,
                            fontSize = 11.sp,
                            fontFamily = FontFamily.Monospace,
                            color = TerminalTextMuted,
                            modifier = Modifier.weight(1f)
                        )
                    }

                    HorizontalDivider(color = TerminalBorder)

                    LazyColumn(modifier = Modifier.weight(1f)) {
                        items(dirList) { dir ->
                            Row(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .clickable { loadDir(dir.path) }
                                    .padding(vertical = 8.dp, horizontal = 4.dp),
                                verticalAlignment = Alignment.CenterVertically
                            ) {
                                Icon(Icons.Default.Folder, contentDescription = null, tint = TerminalAccent)
                                Spacer(modifier = Modifier.width(8.dp))
                                Text(
                                    text = dir.name,
                                    color = TerminalText,
                                    fontSize = 13.sp,
                                    fontFamily = FontFamily.Monospace
                                )
                            }
                        }
                    }

                    Button(
                        onClick = { browsingDirs = false },
                        modifier = Modifier.fillMaxWidth().padding(top = 8.dp),
                        colors = ButtonDefaults.buttonColors(containerColor = TerminalGreen)
                    ) {
                        Text("Use This Folder", color = TerminalBackground)
                    }
                }
            } else {
                Column(modifier = Modifier.fillMaxWidth()) {
                    OutlinedTextField(
                        value = name,
                        onValueChange = { name = it },
                        label = { Text("Session Name (optional)") },
                        singleLine = true,
                        colors = OutlinedTextFieldDefaults.colors(
                            focusedTextColor = TerminalText,
                            unfocusedTextColor = TerminalText,
                            focusedBorderColor = TerminalAccent,
                            unfocusedBorderColor = TerminalBorder
                        ),
                        modifier = Modifier.fillMaxWidth()
                    )

                    Spacer(modifier = Modifier.height(10.dp))

                    OutlinedTextField(
                        value = cwd,
                        onValueChange = { cwd = it },
                        label = { Text("Working Directory") },
                        singleLine = true,
                        colors = OutlinedTextFieldDefaults.colors(
                            focusedTextColor = TerminalText,
                            unfocusedTextColor = TerminalText,
                            focusedBorderColor = TerminalAccent,
                            unfocusedBorderColor = TerminalBorder
                        ),
                        modifier = Modifier.fillMaxWidth(),
                        trailingIcon = {
                            IconButton(onClick = { browsingDirs = true }) {
                                Icon(Icons.Default.Folder, contentDescription = "Browse", tint = TerminalAccent)
                            }
                        }
                    )

                    Spacer(modifier = Modifier.height(12.dp))

                    Text("Preset / Tool:", style = MaterialTheme.typography.bodySmall, color = TerminalTextMuted)
                    Spacer(modifier = Modifier.height(6.dp))
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(6.dp)
                    ) {
                        FilterChip(
                            selected = selectedPreset == "claude",
                            onClick = { selectedPreset = "claude" },
                            label = { Text("🤖 Claude") },
                            colors = FilterChipDefaults.filterChipColors(
                                selectedContainerColor = TerminalPurple.copy(alpha = 0.25f),
                                selectedLabelColor = TerminalPurpleBright
                            )
                        )
                        FilterChip(
                            selected = selectedPreset == "server",
                            onClick = { selectedPreset = "server" },
                            label = { Text("⚙️ Backend") },
                            colors = FilterChipDefaults.filterChipColors(
                                selectedContainerColor = TerminalAmber.copy(alpha = 0.25f),
                                selectedLabelColor = TerminalAmberBright
                            )
                        )
                        FilterChip(
                            selected = selectedPreset == "shell",
                            onClick = { selectedPreset = "shell" },
                            label = { Text("🐚 Shell") },
                            colors = FilterChipDefaults.filterChipColors(
                                selectedContainerColor = TerminalAccent.copy(alpha = 0.25f),
                                selectedLabelColor = TerminalAccent
                            )
                        )
                        FilterChip(
                            selected = selectedPreset == "custom",
                            onClick = { selectedPreset = "custom" },
                            label = { Text("Custom") }
                        )
                    }

                    if (selectedPreset == "custom") {
                        Spacer(modifier = Modifier.height(8.dp))
                        OutlinedTextField(
                            value = customCommand,
                            onValueChange = { customCommand = it },
                            label = { Text("Command Line (e.g. bash ./scripts/...)") },
                            singleLine = true,
                            colors = OutlinedTextFieldDefaults.colors(
                                focusedTextColor = TerminalText,
                                unfocusedTextColor = TerminalText,
                                focusedBorderColor = TerminalAccent,
                                unfocusedBorderColor = TerminalBorder
                            ),
                            modifier = Modifier.fillMaxWidth()
                        )
                    }
                }
            }
        },
        confirmButton = {
            if (!browsingDirs) {
                Button(
                    onClick = {
                        val (cmd, role, defaultTitle) = when (selectedPreset) {
                            "claude" -> Triple("claude", "claude", "Claude Code")
                            "server" -> Triple(
                                "powershell.exe -NoExit -Command \"bash ./scripts/start-backend.sh\"",
                                "server",
                                "⚙️ Project Server"
                            )
                            "shell" -> Triple("powershell.exe", "shell", "PowerShell")
                            else -> Triple(customCommand.ifBlank { "powershell.exe" }, "shell", "Terminal")
                        }
                        val finalTitle = name.ifBlank { defaultTitle }
                        onCreated(
                            CreateSessionRequest(
                                name = finalTitle,
                                cwd = cwd,
                                command = cmd,
                                bubbleId = bubbleId.ifBlank { null },
                                role = role,
                                cols = 100,
                                rows = 30
                            )
                        )
                    },
                    colors = ButtonDefaults.buttonColors(containerColor = TerminalAccent)
                ) {
                    Text("Start Session", color = TerminalBackground)
                }
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) {
                Text("Cancel", color = TerminalTextMuted)
            }
        }
    )
}
