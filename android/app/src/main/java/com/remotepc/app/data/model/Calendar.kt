package com.remotepc.app.data.model

import kotlinx.serialization.Serializable

@Serializable
data class HandoffItem(
    val id: String = "",
    val date: String = "",
    val time: String = "",
    val bubbleId: String = "",
    val taskName: String = "",
    val agent: String = "developer",
    val status: String = "in_progress",
    val summary: String = "",
    val pending: String = "",
    val blockers: String = "",
    val branch: String = "",
    val lastCommit: String = "",
    val dirtyFiles: Int = 0
)

@Serializable
data class DayNote(
    val date: String = "",
    val content: String = "",
    val handoffs: List<HandoffItem> = emptyList(),
    val exists: Boolean = false,
    val path: String = ""
)

@Serializable
data class StandupSummary(
    val referenceDate: String = "",
    val pendingItems: List<HandoffItem> = emptyList()
)

@Serializable
data class AddTodoRequest(
    val text: String,
    val date: String = "",
    val checked: Boolean = false
)

@Serializable
data class AppendNoteRequest(
    val text: String,
    val date: String = ""
)

@Serializable
data class SaveNoteRequest(
    val date: String,
    val content: String
)

data class ParsedTodo(
    val text: String,
    val checked: Boolean,
    val lineNumber: Int
)
