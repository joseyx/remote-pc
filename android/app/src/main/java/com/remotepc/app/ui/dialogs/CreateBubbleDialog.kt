package com.remotepc.app.ui.dialogs

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowDropDown
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.remotepc.app.data.model.*
import com.remotepc.app.data.network.ApiClient
import com.remotepc.app.ui.theme.*
import kotlinx.coroutines.launch

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CreateBubbleDialog(
    apiClient: ApiClient? = null,
    onDismiss: () -> Unit,
    onCreated: (CreateBubbleRequest) -> Unit
) {
    val scope = rememberCoroutineScope()
    var workspace by remember { mutableStateOf("aditum") }
    var branchMode by remember { mutableStateOf("new") } // "new" or "clone"
    var taskId by remember { mutableStateOf("") }
    var name by remember { mutableStateOf("") }
    var baseBranch by remember { mutableStateOf("development") }
    var selectedBranch by remember { mutableStateOf("") }

    // Seed mode for Aditum: "slim", "full", "dump"
    var seedMode by remember { mutableStateOf("slim") }
    var customDumpPath by remember { mutableStateOf("") }

    // Remote branches & SQL dumps
    var branches by remember { mutableStateOf<List<BranchInfo>>(emptyList()) }
    var branchesLoading by remember { mutableStateOf(false) }
    var branchMenuExpanded by remember { mutableStateOf(false) }

    var sqlDumps by remember { mutableStateOf<List<SqlDumpInfo>>(emptyList()) }
    var dumpsLoading by remember { mutableStateOf(false) }
    var dumpsMenuExpanded by remember { mutableStateOf(false) }

    // Function to load branches
    fun loadBranches(ws: String) {
        if (apiClient == null) return
        branchesLoading = true
        scope.launch {
            try {
                branches = apiClient.getWorkspaceBranches(ws)
            } catch (e: Exception) {
                branches = emptyList()
            } finally {
                branchesLoading = false
            }
        }
    }

    // Function to load dumps
    fun loadDumps() {
        if (apiClient == null) return
        dumpsLoading = true
        scope.launch {
            try {
                sqlDumps = apiClient.listSqlDumps()
            } catch (e: Exception) {
                sqlDumps = emptyList()
            } finally {
                dumpsLoading = false
            }
        }
    }

    LaunchedEffect(workspace) {
        if (branchMode == "clone") {
            loadBranches(workspace)
        }
    }

    AlertDialog(
        onDismissRequest = onDismiss,
        containerColor = TerminalSurface,
        title = {
            Text(
                text = "✨ Nueva Tarea / Bubble",
                color = TerminalText,
                style = MaterialTheme.typography.titleMedium
            )
        },
        text = {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .verticalScroll(rememberScrollState()),
                verticalArrangement = Arrangement.spacedBy(10.dp)
            ) {
                Text(
                    text = "Crea un entorno y worktree aislado en tu máquina.",
                    color = TerminalTextMuted,
                    fontSize = 12.sp
                )

                // Workspace Selector
                Text("Proyecto / Workspace", color = TerminalTextDim, fontSize = 11.sp)
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    FilterChip(
                        selected = workspace == "aditum",
                        onClick = {
                            workspace = "aditum"
                            baseBranch = "development"
                            if (branchMode == "clone") loadBranches("aditum")
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
                            baseBranch = "develop"
                            if (branchMode == "clone") loadBranches("brif")
                        },
                        label = { Text("Brif") },
                        colors = FilterChipDefaults.filterChipColors(
                            selectedContainerColor = TerminalPurple.copy(alpha = 0.2f),
                            selectedLabelColor = TerminalPurple
                        )
                    )
                }

                // Branch Origin Selector
                Text("Origen de la Rama", color = TerminalTextDim, fontSize = 11.sp)
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    FilterChip(
                        selected = branchMode == "new",
                        onClick = {
                            branchMode = "new"
                            selectedBranch = ""
                        },
                        label = { Text("Nueva rama") }
                    )
                    FilterChip(
                        selected = branchMode == "clone",
                        onClick = {
                            branchMode = "clone"
                            loadBranches(workspace)
                        },
                        label = { Text("Clonar existente") }
                    )
                }

                if (branchMode == "clone") {
                    // Existing Branch dropdown
                    Column {
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            modifier = Modifier.fillMaxWidth()
                        ) {
                            Text(
                                "Seleccionar Rama del Repo",
                                color = TerminalTextDim,
                                fontSize = 11.sp,
                                modifier = Modifier.weight(1f)
                            )
                            if (branchesLoading) {
                                Text("Cargando...", color = TerminalAccent, fontSize = 10.sp)
                            } else {
                                IconButton(
                                    onClick = { loadBranches(workspace) },
                                    modifier = Modifier.size(24.dp)
                                ) {
                                    Icon(
                                        Icons.Default.Refresh,
                                        contentDescription = "Recargar ramas",
                                        tint = TerminalAccent,
                                        modifier = Modifier.size(16.dp)
                                    )
                                }
                            }
                        }

                        ExposedDropdownMenuBox(
                            expanded = branchMenuExpanded,
                            onExpandedChange = { branchMenuExpanded = !branchMenuExpanded }
                        ) {
                            OutlinedTextField(
                                value = selectedBranch.ifEmpty { "Tocar para elegir rama (${branches.size})" },
                                onValueChange = {},
                                readOnly = true,
                                trailingIcon = {
                                    ExposedDropdownMenuDefaults.TrailingIcon(expanded = branchMenuExpanded)
                                },
                                modifier = Modifier
                                    .menuAnchor()
                                    .fillMaxWidth(),
                                colors = OutlinedTextFieldDefaults.colors(
                                    focusedTextColor = TerminalText,
                                    unfocusedTextColor = TerminalText,
                                    focusedBorderColor = TerminalAccent,
                                    unfocusedBorderColor = TerminalBorder
                                )
                            )
                            ExposedDropdownMenu(
                                expanded = branchMenuExpanded,
                                onDismissRequest = { branchMenuExpanded = false }
                            ) {
                                if (branches.isEmpty()) {
                                    DropdownMenuItem(
                                        text = { Text("No se encontraron ramas") },
                                        onClick = { branchMenuExpanded = false }
                                    )
                                } else {
                                    branches.take(50).forEach { branch ->
                                        DropdownMenuItem(
                                            text = {
                                                Column {
                                                    Text(branch.name, color = TerminalText, fontSize = 13.sp)
                                                    Text(
                                                        if (branch.isRemote) "remota (origin)" else "local",
                                                        color = TerminalTextDim,
                                                        fontSize = 10.sp
                                                    )
                                                }
                                            },
                                            onClick = {
                                                selectedBranch = branch.name
                                                taskId = branch.name
                                                if (name.isEmpty()) name = branch.name
                                                branchMenuExpanded = false
                                            }
                                        )
                                    }
                                }
                            }
                        }
                    }
                }

                // Task ID
                OutlinedTextField(
                    value = taskId,
                    onValueChange = {
                        taskId = it
                        if (name.isEmpty()) name = it
                    },
                    label = { Text(if (workspace == "aditum") "ID de Tarea / Rama (ej. TAR-1180)" else "Nombre Tarea (ej. dialogos-mx)") },
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
                if (branchMode == "new") {
                    OutlinedTextField(
                        value = baseBranch,
                        onValueChange = { baseBranch = it },
                        label = { Text("Rama Base (ej. development)") },
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

                // Database seed options for Aditum
                if (workspace == "aditum") {
                    Text("Semilla MySQL 8.0", color = TerminalTextDim, fontSize = 11.sp)
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(6.dp)
                    ) {
                        FilterChip(
                            selected = seedMode == "slim",
                            onClick = { seedMode = "slim" },
                            label = { Text("Slim (~25s)") },
                            colors = FilterChipDefaults.filterChipColors(
                                selectedContainerColor = TerminalAccent.copy(alpha = 0.2f),
                                selectedLabelColor = TerminalAccent
                            )
                        )
                        FilterChip(
                            selected = seedMode == "full",
                            onClick = { seedMode = "full" },
                            label = { Text("Full (3.5 GB)") }
                        )
                        FilterChip(
                            selected = seedMode == "dump",
                            onClick = {
                                seedMode = "dump"
                                loadDumps()
                            },
                            label = { Text("Dump SQL") }
                        )
                    }

                    if (seedMode == "dump") {
                        Column(
                            verticalArrangement = Arrangement.spacedBy(6.dp),
                            modifier = Modifier.fillMaxWidth()
                        ) {
                            Row(
                                verticalAlignment = Alignment.CenterVertically,
                                modifier = Modifier.fillMaxWidth()
                            ) {
                                Text(
                                    "Dumps Detectados en Descargas / Work",
                                    color = TerminalTextDim,
                                    fontSize = 11.sp,
                                    modifier = Modifier.weight(1f)
                                )
                                if (dumpsLoading) {
                                    Text("Buscando...", color = TerminalAccent, fontSize = 10.sp)
                                } else {
                                    IconButton(
                                        onClick = { loadDumps() },
                                        modifier = Modifier.size(24.dp)
                                    ) {
                                        Icon(
                                            Icons.Default.Refresh,
                                            contentDescription = "Buscar dumps",
                                            tint = TerminalAccent,
                                            modifier = Modifier.size(16.dp)
                                        )
                                    }
                                }
                            }

                            ExposedDropdownMenuBox(
                                expanded = dumpsMenuExpanded,
                                onExpandedChange = { dumpsMenuExpanded = !dumpsMenuExpanded }
                            ) {
                                OutlinedTextField(
                                    value = customDumpPath.ifEmpty { "Elegir dump detectado (${sqlDumps.size})" },
                                    onValueChange = {},
                                    readOnly = true,
                                    trailingIcon = {
                                        ExposedDropdownMenuDefaults.TrailingIcon(expanded = dumpsMenuExpanded)
                                    },
                                    modifier = Modifier
                                        .menuAnchor()
                                        .fillMaxWidth(),
                                    colors = OutlinedTextFieldDefaults.colors(
                                        focusedTextColor = TerminalText,
                                        unfocusedTextColor = TerminalText,
                                        focusedBorderColor = TerminalAccent,
                                        unfocusedBorderColor = TerminalBorder
                                    )
                                )
                                ExposedDropdownMenu(
                                    expanded = dumpsMenuExpanded,
                                    onDismissRequest = { dumpsMenuExpanded = false }
                                ) {
                                    if (sqlDumps.isEmpty()) {
                                        DropdownMenuItem(
                                            text = { Text("No se encontraron dumps en Descargas") },
                                            onClick = { dumpsMenuExpanded = false }
                                        )
                                    } else {
                                        sqlDumps.forEach { dump ->
                                            DropdownMenuItem(
                                                text = {
                                                    Column {
                                                        Text(dump.name, color = TerminalText, fontSize = 13.sp)
                                                        Text("${dump.sizeStr} • ${dump.path}", color = TerminalTextDim, fontSize = 10.sp)
                                                    }
                                                },
                                                onClick = {
                                                    customDumpPath = dump.path
                                                    dumpsMenuExpanded = false
                                                }
                                            )
                                        }
                                    }
                                }
                            }

                            OutlinedTextField(
                                value = customDumpPath,
                                onValueChange = { customDumpPath = it },
                                label = { Text("Ruta del archivo .sql") },
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
            }
        },
        confirmButton = {
            Button(
                onClick = {
                    val finalTaskId = if (branchMode == "clone") selectedBranch.ifEmpty { taskId } else taskId
                    if (finalTaskId.isNotBlank()) {
                        onCreated(
                            CreateBubbleRequest(
                                workspace = workspace,
                                taskId = finalTaskId.trim(),
                                name = name.ifBlank { finalTaskId }.trim(),
                                baseBranch = baseBranch.trim(),
                                cloneBranch = if (branchMode == "clone") selectedBranch.trim() else "",
                                useSlim = seedMode == "slim",
                                seedDump = if (seedMode == "dump") customDumpPath.trim() else "",
                                isWorktree = true
                            )
                        )
                    }
                },
                enabled = (if (branchMode == "clone") selectedBranch.isNotBlank() || taskId.isNotBlank() else taskId.isNotBlank()) &&
                        (seedMode != "dump" || customDumpPath.isNotBlank()),
                colors = ButtonDefaults.buttonColors(containerColor = TerminalAccent)
            ) {
                Text("Crear Bubble", color = TerminalBackground)
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) {
                Text("Cancelar", color = TerminalTextMuted)
            }
        }
    )
}
