package com.remotepc.app.data.model

import kotlinx.serialization.Serializable

@Serializable
data class Bubble(
    val id: String,
    val name: String,
    val workspace: String = "general",
    val path: String = "",
    val branch: String = "",
    val baseBranch: String = "",
    val port: Int = 0,
    val dbSchema: String = "",
    val status: String = "idle",
    val createdAt: String = "",
    val isWorktree: Boolean = false,
    val devRunning: Boolean = false,
    val devPid: Int = 0,
    val priority: String = "",
    val customRank: Int = 0
)

@Serializable
data class CreateBubbleRequest(
    val workspace: String,
    val taskId: String,
    val name: String = "",
    val baseBranch: String = "",
    val useSlim: Boolean = true,
    val isWorktree: Boolean = true
)

@Serializable
data class GitChange(
    val path: String = "",
    val status: String = "M",
    val staged: Boolean = false
)

@Serializable
data class GitStatusResponse(
    val branch: String = "",
    val isDirty: Boolean = false,
    val ahead: Int = 0,
    val behind: Int = 0,
    val changes: List<GitChange> = emptyList()
)
