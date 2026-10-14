package com.remotepc.app.ui.daily

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
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
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.remotepc.app.data.model.DayNote
import com.remotepc.app.data.model.HandoffItem
import com.remotepc.app.data.model.ParsedTodo
import com.remotepc.app.data.model.StandupSummary
import com.remotepc.app.data.network.ApiClient
import com.remotepc.app.ui.theme.*
import kotlinx.coroutines.launch
import java.text.SimpleDateFormat
import java.util.*

@Composable
fun DailyScreen(
    apiClient: ApiClient
) {
    val scope = rememberCoroutineScope()
    val dateFormat = remember { SimpleDateFormat("yyyy-MM-dd", Locale.getDefault()) }
    val displayFormat = remember { SimpleDateFormat("EEEE, MMM d", Locale.getDefault()) }

    var selectedDate by remember { mutableStateOf(dateFormat.format(Date())) }
    var dayNote by remember { mutableStateOf<DayNote?>(null) }
    var standup by remember { mutableStateOf<StandupSummary?>(null) }
    var isLoading by remember { mutableStateOf(false) }
    var showStandupDialog by remember { mutableStateOf(false) }

    var newTodoText by remember { mutableStateOf("") }

    fun refreshDay() {
        scope.launch {
            isLoading = true
            try {
                dayNote = apiClient.getDayNote(selectedDate)
                standup = apiClient.getStandup()
            } catch (e: Exception) {
                // handle error
            } finally {
                isLoading = false
            }
        }
    }

    LaunchedEffect(selectedDate) {
        refreshDay()
    }

    // Parse todos from markdown content (- [ ] task or - [x] task)
    val parsedTodos = remember(dayNote?.content) {
        val list = mutableListOf<ParsedTodo>()
        val lines = dayNote?.content?.lines() ?: emptyList()
        lines.forEachIndexed { index, line ->
            val trimmed = line.trim()
            if (trimmed.startsWith("- [ ]") || trimmed.startsWith("- [x]") || trimmed.startsWith("- [X]")) {
                val checked = trimmed.startsWith("- [x]") || trimmed.startsWith("- [X]")
                val text = trimmed.substring(5).trim()
                if (text.isNotBlank()) {
                    list.add(ParsedTodo(text = text, checked = checked, lineNumber = index))
                }
            }
        }
        list
    }

    val todayStr = remember { dateFormat.format(Date()) }
    val isToday = selectedDate == todayStr

    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(horizontal = 14.dp, vertical = 6.dp)
    ) {
        // Date Selector & Standup Shortcut
        Row(
            modifier = Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Surface(
                    color = if (isToday) TerminalAccent.copy(alpha = 0.2f) else TerminalSurface,
                    shape = RoundedCornerShape(8.dp),
                    border = BorderStroke(1.dp, if (isToday) TerminalAccent else TerminalBorder),
                    modifier = Modifier.clickable { selectedDate = todayStr }
                ) {
                    Text(
                        text = "Today",
                        color = if (isToday) TerminalAccent else TerminalTextMuted,
                        fontWeight = FontWeight.Bold,
                        fontSize = 12.sp,
                        modifier = Modifier.padding(horizontal = 10.dp, vertical = 6.dp)
                    )
                }

                Spacer(modifier = Modifier.width(8.dp))

                val cal = Calendar.getInstance()
                cal.add(Calendar.DAY_OF_YEAR, -1)
                val yesterdayStr = dateFormat.format(cal.time)
                val isYesterday = selectedDate == yesterdayStr

                Surface(
                    color = if (isYesterday) TerminalAccent.copy(alpha = 0.2f) else TerminalSurface,
                    shape = RoundedCornerShape(8.dp),
                    border = BorderStroke(1.dp, if (isYesterday) TerminalAccent else TerminalBorder),
                    modifier = Modifier.clickable { selectedDate = yesterdayStr }
                ) {
                    Text(
                        text = "Yesterday",
                        color = if (isYesterday) TerminalAccent else TerminalTextMuted,
                        fontWeight = FontWeight.Bold,
                        fontSize = 12.sp,
                        modifier = Modifier.padding(horizontal = 10.dp, vertical = 6.dp)
                    )
                }
            }

            // Standup pending button
            FilledTonalButton(
                onClick = { showStandupDialog = true },
                shape = RoundedCornerShape(8.dp),
                contentPadding = PaddingValues(horizontal = 10.dp, vertical = 6.dp),
                colors = ButtonDefaults.filledTonalButtonColors(containerColor = TerminalPurple.copy(alpha = 0.2f))
            ) {
                Icon(Icons.Default.Bolt, contentDescription = null, tint = TerminalPurpleBright, modifier = Modifier.size(14.dp))
                Spacer(modifier = Modifier.width(4.dp))
                Text("Standup", color = TerminalPurpleBright, fontSize = 11.sp, fontWeight = FontWeight.Bold)
            }
        }

        Spacer(modifier = Modifier.height(14.dp))

        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            verticalArrangement = Arrangement.spacedBy(14.dp),
            contentPadding = PaddingValues(bottom = 80.dp)
        ) {
            // Section 1: Quick To-Dos
            item {
                Card(
                    shape = RoundedCornerShape(14.dp),
                    colors = CardDefaults.cardColors(containerColor = TerminalSurface),
                    border = BorderStroke(1.dp, TerminalBorder),
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Column(modifier = Modifier.padding(14.dp)) {
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.SpaceBetween,
                            modifier = Modifier.fillMaxWidth()
                        ) {
                            Row(verticalAlignment = Alignment.CenterVertically) {
                                Icon(Icons.Default.CheckCircleOutline, contentDescription = null, tint = TerminalAccent, modifier = Modifier.size(16.dp))
                                Spacer(modifier = Modifier.width(6.dp))
                                Text("Daily Checklist", color = TerminalText, fontWeight = FontWeight.Bold, fontSize = 14.sp)
                            }
                            Text("${parsedTodos.count { it.checked }}/${parsedTodos.size} Done", color = TerminalTextDim, fontSize = 11.sp)
                        }

                        Spacer(modifier = Modifier.height(10.dp))

                        // Quick Add Input
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            modifier = Modifier.fillMaxWidth()
                        ) {
                            OutlinedTextField(
                                value = newTodoText,
                                onValueChange = { newTodoText = it },
                                placeholder = { Text("Add task for today...", color = TerminalTextDim, fontSize = 12.sp) },
                                singleLine = true,
                                shape = RoundedCornerShape(10.dp),
                                colors = OutlinedTextFieldDefaults.colors(
                                    focusedTextColor = TerminalText,
                                    unfocusedTextColor = TerminalText,
                                    focusedContainerColor = TerminalSurfaceElevated,
                                    unfocusedContainerColor = TerminalSurfaceElevated,
                                    focusedBorderColor = TerminalAccent,
                                    unfocusedBorderColor = TerminalBorder
                                ),
                                modifier = Modifier
                                    .weight(1f)
                                    .height(46.dp)
                            )
                            Spacer(modifier = Modifier.width(8.dp))
                            IconButton(
                                onClick = {
                                    if (newTodoText.isNotBlank()) {
                                        scope.launch {
                                            apiClient.addDailyTodo(newTodoText.trim(), selectedDate, false)
                                            newTodoText = ""
                                            refreshDay()
                                        }
                                    }
                                },
                                modifier = Modifier
                                    .size(46.dp)
                                    .clip(RoundedCornerShape(10.dp))
                                    .background(TerminalAccent)
                            ) {
                                Icon(Icons.Default.Add, contentDescription = "Add", tint = TerminalBackground)
                            }
                        }

                        if (parsedTodos.isNotEmpty()) {
                            Spacer(modifier = Modifier.height(10.dp))
                            parsedTodos.forEach { todo ->
                                Row(
                                    verticalAlignment = Alignment.CenterVertically,
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .clickable {
                                            scope.launch {
                                                apiClient.addDailyTodo(todo.text, selectedDate, !todo.checked)
                                                refreshDay()
                                            }
                                        }
                                        .padding(vertical = 4.dp)
                                ) {
                                    Checkbox(
                                        checked = todo.checked,
                                        onCheckedChange = { checked ->
                                            scope.launch {
                                                apiClient.addDailyTodo(todo.text, selectedDate, checked)
                                                refreshDay()
                                            }
                                        },
                                        colors = CheckboxDefaults.colors(checkedColor = TerminalAccent)
                                    )
                                    Spacer(modifier = Modifier.width(6.dp))
                                    Text(
                                        text = todo.text,
                                        color = if (todo.checked) TerminalTextDim else TerminalText,
                                        fontSize = 13.sp,
                                        textDecoration = if (todo.checked) TextDecoration.LineThrough else TextDecoration.None
                                    )
                                }
                            }
                        }
                    }
                }
            }

            // Section 2: Handoffs of the day
            val handoffs = dayNote?.handoffs ?: emptyList()
            item {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Icon(Icons.Default.BookmarkBorder, contentDescription = null, tint = TerminalPurpleBright, modifier = Modifier.size(16.dp))
                    Spacer(modifier = Modifier.width(6.dp))
                    Text("Daily Task Handoffs (${handoffs.size})", color = TerminalText, fontWeight = FontWeight.Bold, fontSize = 14.sp)
                }
            }

            if (handoffs.isEmpty()) {
                item {
                    Surface(
                        color = TerminalSurface,
                        shape = RoundedCornerShape(12.dp),
                        border = BorderStroke(1.dp, TerminalBorder),
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Column(
                            horizontalAlignment = Alignment.CenterHorizontally,
                            modifier = Modifier.padding(20.dp)
                        ) {
                            Text("No handoffs registered for $selectedDate.", color = TerminalTextMuted, fontSize = 13.sp)
                            Spacer(modifier = Modifier.height(4.dp))
                            Text("Ask Claude or Orbit MCP to 'cerrar el día' to record progress.", color = TerminalTextDim, fontSize = 11.sp)
                        }
                    }
                }
            } else {
                items(handoffs) { handoff ->
                    HandoffCard(handoff = handoff)
                }
            }
        }
    }

    // Standup Sheet/Dialog
    if (showStandupDialog) {
        AlertDialog(
            onDismissRequest = { showStandupDialog = false },
            containerColor = TerminalSurface,
            title = {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Icon(Icons.Default.Bolt, contentDescription = null, tint = TerminalPurpleBright)
                    Spacer(modifier = Modifier.width(8.dp))
                    Text("Pending From Yesterday", color = TerminalText, style = MaterialTheme.typography.titleMedium)
                }
            },
            text = {
                val pendingList = standup?.pendingItems ?: emptyList()
                if (pendingList.isEmpty()) {
                    Text("No pending items registered from yesterday. You are clear to start fresh!", color = TerminalTextMuted, fontSize = 13.sp)
                } else {
                    LazyColumn(
                        modifier = Modifier
                            .fillMaxWidth()
                            .heightIn(max = 350.dp),
                        verticalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        items(pendingList) { item ->
                            Card(
                                shape = RoundedCornerShape(10.dp),
                                colors = CardDefaults.cardColors(containerColor = TerminalSurfaceElevated)
                            ) {
                                Column(modifier = Modifier.padding(10.dp)) {
                                    Text(item.taskName, color = TerminalAccent, fontWeight = FontWeight.Bold, fontSize = 13.sp)
                                    Spacer(modifier = Modifier.height(4.dp))
                                    Text(item.pending, color = TerminalText, fontSize = 12.sp)
                                }
                            }
                        }
                    }
                }
            },
            confirmButton = {
                Button(
                    onClick = { showStandupDialog = false },
                    colors = ButtonDefaults.buttonColors(containerColor = TerminalAccent)
                ) {
                    Text("Got It", color = TerminalBackground)
                }
            }
        )
    }
}

@Composable
private fun HandoffCard(handoff: HandoffItem) {
    Card(
        shape = RoundedCornerShape(14.dp),
        colors = CardDefaults.cardColors(containerColor = TerminalSurface),
        border = BorderStroke(1.dp, TerminalBorder),
        modifier = Modifier.fillMaxWidth()
    ) {
        Column(modifier = Modifier.padding(14.dp)) {
            // Header: Task name, Agent badge, Status badge
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        text = handoff.taskName,
                        color = TerminalText,
                        fontSize = 15.sp,
                        fontWeight = FontWeight.Bold
                    )
                    Spacer(modifier = Modifier.width(8.dp))
                    Surface(
                        color = TerminalPurple.copy(alpha = 0.2f),
                        shape = RoundedCornerShape(4.dp)
                    ) {
                        Text(
                            text = handoff.agent.uppercase(),
                            color = TerminalPurpleBright,
                            fontSize = 10.sp,
                            fontWeight = FontWeight.ExtraBold,
                            modifier = Modifier.padding(horizontal = 5.dp, vertical = 2.dp)
                        )
                    }
                }

                val (statusColor, statusLabel) = when (handoff.status.lowercase()) {
                    "completed" -> TerminalGreen to "DONE"
                    "blocked" -> TerminalRed to "BLOCKED"
                    else -> TerminalAmber to "IN PROGRESS"
                }

                Surface(
                    color = statusColor.copy(alpha = 0.15f),
                    shape = RoundedCornerShape(4.dp)
                ) {
                    Text(
                        text = statusLabel,
                        color = statusColor,
                        fontSize = 10.sp,
                        fontWeight = FontWeight.Bold,
                        modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp)
                    )
                }
            }

            Spacer(modifier = Modifier.height(10.dp))

            // Summary
            Text("Completed Today:", color = TerminalTextDim, fontSize = 11.sp, fontWeight = FontWeight.Bold)
            Spacer(modifier = Modifier.height(2.dp))
            Text(handoff.summary, color = TerminalText, fontSize = 12.sp)

            // Pending
            if (handoff.pending.isNotBlank()) {
                Spacer(modifier = Modifier.height(8.dp))
                Text("Pending Tomorrow:", color = TerminalTextDim, fontSize = 11.sp, fontWeight = FontWeight.Bold)
                Spacer(modifier = Modifier.height(2.dp))
                Text(handoff.pending, color = TerminalAccent, fontSize = 12.sp)
            }

            // Blockers
            if (handoff.blockers.isNotBlank()) {
                Spacer(modifier = Modifier.height(8.dp))
                Surface(
                    color = TerminalRed.copy(alpha = 0.1f),
                    shape = RoundedCornerShape(6.dp),
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Row(modifier = Modifier.padding(8.dp), verticalAlignment = Alignment.CenterVertically) {
                        Icon(Icons.Default.Warning, contentDescription = null, tint = TerminalRed, modifier = Modifier.size(14.dp))
                        Spacer(modifier = Modifier.width(6.dp))
                        Text("Blocker: ${handoff.blockers}", color = TerminalRedBright, fontSize = 11.sp)
                    }
                }
            }
        }
    }
}
