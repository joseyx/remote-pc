package com.remotepc.app.ui.dialogs

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Folder
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FilterChipDefaults
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.remotepc.app.data.model.CreateSessionRequest
import com.remotepc.app.data.model.DirItem
import com.remotepc.app.data.network.ApiClient
import com.remotepc.app.ui.theme.TerminalAccent
import com.remotepc.app.ui.theme.TerminalGreen
import com.remotepc.app.ui.theme.TerminalSurface
import com.remotepc.app.ui.theme.TerminalText
import com.remotepc.app.ui.theme.TerminalTextMuted
import kotlinx.coroutines.launch

@Composable
fun CreateSessionDialog(
    apiClient: ApiClient,
    onDismiss: () -> Unit,
    onCreated: (CreateSessionRequest) -> Unit
) {
    var name by remember { mutableStateOf("") }
    var cwd by remember { mutableStateOf("") }
    var selectedCommand by remember { mutableStateOf("claude") }
    var customCommand by remember { mutableStateOf("") }

    var browsingDirs by remember { mutableStateOf(false) }
    var currentDirPath by remember { mutableStateOf("") }
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
            loadDir("")
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

                    HorizontalDivider(color = MaterialTheme.colorScheme.surfaceVariant)

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
                                Spacer(modifier = Modifier.padding(horizontal = 4.dp))
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
                        Text("Use This Folder", color = TerminalSurface)
                    }
                }
            } else {
                Column(modifier = Modifier.fillMaxWidth()) {
                    OutlinedTextField(
                        value = name,
                        onValueChange = { name = it },
                        label = { Text("Session Name (e.g. claude-api)") },
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth()
                    )

                    Spacer(modifier = Modifier.height(10.dp))

                    OutlinedTextField(
                        value = cwd,
                        onValueChange = { cwd = it },
                        label = { Text("Working Directory (PC path)") },
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth(),
                        trailingIcon = {
                            IconButton(onClick = { browsingDirs = true }) {
                                Icon(Icons.Default.Folder, contentDescription = "Browse", tint = TerminalAccent)
                            }
                        }
                    )

                    Spacer(modifier = Modifier.height(12.dp))

                    Text("Command:", style = MaterialTheme.typography.bodySmall, color = TerminalTextMuted)
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(6.dp)
                    ) {
                        FilterChip(
                            selected = selectedCommand == "claude",
                            onClick = { selectedCommand = "claude" },
                            label = { Text("claude") },
                            colors = FilterChipDefaults.filterChipColors(
                                selectedContainerColor = TerminalAccent.copy(alpha = 0.3f),
                                selectedLabelColor = TerminalAccent
                            )
                        )
                        FilterChip(
                            selected = selectedCommand == "powershell.exe",
                            onClick = { selectedCommand = "powershell.exe" },
                            label = { Text("powershell") }
                        )
                        FilterChip(
                            selected = selectedCommand == "cmd.exe",
                            onClick = { selectedCommand = "cmd.exe" },
                            label = { Text("cmd") }
                        )
                        FilterChip(
                            selected = selectedCommand == "custom",
                            onClick = { selectedCommand = "custom" },
                            label = { Text("custom") }
                        )
                    }

                    if (selectedCommand == "custom") {
                        Spacer(modifier = Modifier.height(8.dp))
                        OutlinedTextField(
                            value = customCommand,
                            onValueChange = { customCommand = it },
                            label = { Text("Custom Command Line") },
                            singleLine = true,
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
                        val finalCmd = if (selectedCommand == "custom") customCommand else selectedCommand
                        val sessionTitle = name.ifBlank {
                            if (finalCmd.contains("claude")) "claude" else "Terminal"
                        }
                        onCreated(
                            CreateSessionRequest(
                                name = sessionTitle,
                                cwd = cwd,
                                command = finalCmd,
                                cols = 100,
                                rows = 30
                            )
                        )
                    },
                    colors = ButtonDefaults.buttonColors(containerColor = TerminalAccent)
                ) {
                    Text("Start Session", color = TerminalSurface)
                }
            }
        },
        dismissButton = {
            OutlinedButton(onClick = onDismiss) {
                Text("Cancel")
            }
        }
    )
}
