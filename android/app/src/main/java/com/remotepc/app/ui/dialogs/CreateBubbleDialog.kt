package com.remotepc.app.ui.dialogs

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.remotepc.app.data.model.CreateBubbleRequest
import com.remotepc.app.ui.theme.*

@Composable
fun CreateBubbleDialog(
    onDismiss: () -> Unit,
    onCreated: (CreateBubbleRequest) -> Unit
) {
    var workspace by remember { mutableStateOf("aditum") }
    var taskId by remember { mutableStateOf("") }
    var name by remember { mutableStateOf("") }
    var baseBranch by remember { mutableStateOf("") }
    var useSlim by remember { mutableStateOf(true) }
    var isWorktree by remember { mutableStateOf(true) }

    AlertDialog(
        onDismissRequest = onDismiss,
        containerColor = TerminalSurface,
        title = {
            Text(
                text = "New Workspace Bubble",
                color = TerminalText,
                style = MaterialTheme.typography.titleMedium
            )
        },
        text = {
            Column(
                modifier = Modifier.fillMaxWidth(),
                verticalArrangement = Arrangement.spacedBy(10.dp)
            ) {
                Text(
                    text = "Create an isolated Git worktree and environment directly from mobile.",
                    color = TerminalTextMuted,
                    fontSize = 12.sp
                )

                // Workspace Selector
                Text("Workspace", color = TerminalTextDim, fontSize = 11.sp)
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    FilterChip(
                        selected = workspace == "aditum",
                        onClick = {
                            workspace = "aditum"
                            if (baseBranch.isEmpty()) baseBranch = "development"
                        },
                        label = { Text("Aditum") },
                        colors = FilterChipDefaults.filterChipColors(
                            selectedContainerColor = TerminalAccent.copy(alpha = 0.2f),
                            selectedLabelColor = TerminalAccent
                        )
                    )
                    FilterChip(
                        selected = workspace == "brif",
                        onClick = {
                            workspace = "brif"
                            if (baseBranch.isEmpty()) baseBranch = "develop"
                        },
                        label = { Text("Brif") },
                        colors = FilterChipDefaults.filterChipColors(
                            selectedContainerColor = TerminalPurple.copy(alpha = 0.2f),
                            selectedLabelColor = TerminalPurple
                        )
                    )
                }

                // Task ID
                OutlinedTextField(
                    value = taskId,
                    onValueChange = {
                        taskId = it
                        if (name.isEmpty()) name = it
                    },
                    label = { Text(if (workspace == "aditum") "Task ID (e.g. TAR-1180)" else "Task Name (e.g. scrap-clarin)") },
                    singleLine = true,
                    colors = OutlinedTextFieldDefaults.colors(
                        focusedTextColor = TerminalText,
                        unfocusedTextColor = TerminalText,
                        focusedBorderColor = TerminalAccent,
                        unfocusedBorderColor = TerminalBorder
                    ),
                    modifier = Modifier.fillMaxWidth()
                )

                // Base branch
                OutlinedTextField(
                    value = baseBranch,
                    onValueChange = { baseBranch = it },
                    label = { Text("Base Branch (e.g. development)") },
                    singleLine = true,
                    colors = OutlinedTextFieldDefaults.colors(
                        focusedTextColor = TerminalText,
                        unfocusedTextColor = TerminalText,
                        focusedBorderColor = TerminalAccent,
                        unfocusedBorderColor = TerminalBorder
                    ),
                    modifier = Modifier.fillMaxWidth()
                )

                // Slim seed switch for Aditum
                if (workspace == "aditum") {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Checkbox(
                            checked = useSlim,
                            onCheckedChange = { useSlim = it },
                            colors = CheckboxDefaults.colors(checkedColor = TerminalAccent)
                        )
                        Spacer(modifier = Modifier.width(4.dp))
                        Column {
                            Text("Use Slim Database Seed", color = TerminalText, fontSize = 12.sp)
                            Text("Fast local schema initialization", color = TerminalTextDim, fontSize = 10.sp)
                        }
                    }
                }
            }
        },
        confirmButton = {
            Button(
                onClick = {
                    if (taskId.isNotBlank()) {
                        onCreated(
                            CreateBubbleRequest(
                                workspace = workspace,
                                taskId = taskId.trim(),
                                name = name.ifBlank { taskId }.trim(),
                                baseBranch = baseBranch.trim(),
                                useSlim = useSlim,
                                isWorktree = isWorktree
                            )
                        )
                    }
                },
                enabled = taskId.isNotBlank(),
                colors = ButtonDefaults.buttonColors(containerColor = TerminalAccent)
            ) {
                Text("Create Bubble", color = TerminalBackground)
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) {
                Text("Cancel", color = TerminalTextMuted)
            }
        }
    )
}
