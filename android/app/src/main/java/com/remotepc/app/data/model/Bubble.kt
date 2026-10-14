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
data class WorkspaceInfo(
    val id: String,
    val name: String,
    val type: String = "",
    val basePath: String = "",
    val vaultAlias: String = "",
    val defaultBase: String = "",
    val description: String = "",
    val devCommand: String = "",
    val defaultPort: Int = 0,
    val repoUrl: String = ""
)

@Serializable
data class BranchInfo(
    val name: String,
    val isRemote: Boolean = false
)

@Serializable
data class SqlDumpInfo(
    val path: String,
    val name: String,
    val size: Long = 0L,
    val sizeStr: String = "",
    val modTime: String = ""
)

@Serializable
data class CreateBubbleRequest(
    val workspace: String,
    val taskId: String,
    val name: String = "",
    val baseBranch: String = "",
    val cloneBranch: String = "",
    val useSlim: Boolean = true,
    val seedSchema: String = "",
    val seedDump: String = "",
    val index: Int = 0,
    val isWorktree: Boolean = true,
    val port: Int = 0
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
