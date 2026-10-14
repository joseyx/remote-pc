package com.remotepc.app.ui.dialogs

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowDropDown
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
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
    var branchSearchQuery by remember { mutableStateOf("") }

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
                    val filteredBranches = remember(branches, branchSearchQuery) {
                        if (branchSearchQuery.isBlank()) {
                            branches.take(80)
                        } else {
                            branches.filter { it.name.contains(branchSearchQuery, ignoreCase = true) }.take(80)
                        }
                    }

                    // Existing Branch dropdown with real-time search
                    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
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
                                Text("Cargando ramas...", color = TerminalAccent, fontSize = 10.sp)
                            } else {
                                Row(verticalAlignment = Alignment.CenterVertically) {
                                    Text("${branches.size} ramas", color = TerminalTextDim, fontSize = 10.sp)
                                    Spacer(Modifier.width(4.dp))
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
                        }

                        ExposedDropdownMenuBox(
                            expanded = branchMenuExpanded,
                            onExpandedChange = { branchMenuExpanded = it }
                        ) {
                            OutlinedTextField(
                                value = branchSearchQuery,
                                onValueChange = {
                                    branchSearchQuery = it
                                    branchMenuExpanded = true
                                },
                                placeholder = {
                                    Text(
                                        if (selectedBranch.isNotEmpty()) selectedBranch else "🔍 Buscar rama (ej. TAR-1098)...",
                                        color = if (selectedBranch.isNotEmpty()) TerminalAccent else TerminalTextDim,
                                        fontSize = 12.sp
                                    )
                                },
                                singleLine = true,
                                trailingIcon = {
                                    if (branchSearchQuery.isNotEmpty()) {
                                        IconButton(onClick = { branchSearchQuery = "" }) {
                                            Icon(
                                                Icons.Default.Close,
                                                contentDescription = "Limpiar búsqueda",
                                                tint = TerminalTextDim,
                                                modifier = Modifier.size(16.dp)
                                            )
                                        }
                                    } else {
                                        ExposedDropdownMenuDefaults.TrailingIcon(expanded = branchMenuExpanded)
                                    }
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
                                if (branchesLoading) {
                                    DropdownMenuItem(
                                        text = { Text("Cargando ramas...", color = TerminalTextDim, fontSize = 12.sp) },
                                        onClick = {}
                                    )
                                } else if (branches.isEmpty()) {
                                    DropdownMenuItem(
                                        text = { Text("No se encontraron ramas en el repositorio", color = TerminalTextDim, fontSize = 12.sp) },
                                        onClick = { branchMenuExpanded = false }
                                    )
                                } else if (filteredBranches.isEmpty()) {
                                    DropdownMenuItem(
                                        text = { Text("Sin coincidencias para \"$branchSearchQuery\"", color = TerminalTextDim, fontSize = 12.sp) },
                                        onClick = {}
                                    )
                                } else {
                                    filteredBranches.forEach { branch ->
                                        DropdownMenuItem(
                                            text = {
                                                Row(
                                                    modifier = Modifier.fillMaxWidth(),
                                                    horizontalArrangement = Arrangement.SpaceBetween,
                                                    verticalAlignment = Alignment.CenterVertically
                                                ) {
                                                    Text(
                                                        branch.name,
                                                        color = if (branch.name == selectedBranch) TerminalAccent else TerminalText,
                                                        fontSize = 12.sp,
                                                        fontWeight = if (branch.name == selectedBranch) FontWeight.Bold else FontWeight.Normal,
                                                        modifier = Modifier.weight(1f, fill = false)
                                                    )
                                                    Spacer(Modifier.width(8.dp))
                                                    Surface(
                                                        shape = RoundedCornerShape(4.dp),
                                                        color = if (branch.isRemote) TerminalAccent.copy(alpha = 0.15f) else TerminalGreen.copy(alpha = 0.15f)
                                                    ) {
                                                        Text(
                                                            if (branch.isRemote) "origin" else "local",
                                                            color = if (branch.isRemote) TerminalAccent else TerminalGreen,
                                                            fontSize = 10.sp,
                                                            modifier = Modifier.padding(horizontal = 5.dp, vertical = 2.dp)
                                                        )
                                                    }
                                                }
                                            },
                                            onClick = {
                                                selectedBranch = branch.name
                                                branchSearchQuery = branch.name
                                                taskId = branch.name
                                                if (name.isEmpty()) name = branch.name
                                                branchMenuExpanded = false
                                            }
                                        )
                                    }
                                }
                            }
                        }

                        if (selectedBranch.isNotEmpty()) {
                            Surface(
                                shape = RoundedCornerShape(6.dp),
                                color = TerminalAccent.copy(alpha = 0.1f),
                                border = BorderStroke(1.dp, TerminalAccent.copy(alpha = 0.35f)),
                                modifier = Modifier.fillMaxWidth()
                            ) {
                                Row(
                                    modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp),
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.SpaceBetween
                                ) {
                                    Row(
                                        verticalAlignment = Alignment.CenterVertically,
                                        modifier = Modifier.weight(1f)
                                    ) {
                                        Text("Rama elegida: ", color = TerminalTextDim, fontSize = 11.sp)
                                        Text(selectedBranch, color = TerminalAccent, fontSize = 11.sp, fontWeight = FontWeight.Bold)
                                    }
                                    IconButton(
                                        onClick = {
                                            selectedBranch = ""
                                            branchSearchQuery = ""
                                        },
                                        modifier = Modifier.size(20.dp)
                                    ) {
                                        Icon(
                                            Icons.Default.Close,
                                            contentDescription = "Quitar selección",
                                            tint = TerminalAccent,
                                            modifier = Modifier.size(14.dp)
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
