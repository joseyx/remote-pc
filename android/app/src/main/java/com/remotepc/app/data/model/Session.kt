package com.remotepc.app.data.model

import kotlinx.serialization.Serializable

@Serializable
data class Session(
    val id: String,
    val name: String,
    val cwd: String,
    val command: String,
    val pid: Int,
    val isRunning: Boolean,
    val exitCode: Int = 0,
    val createdAt: String = ""
)

@Serializable
data class CreateSessionRequest(
    val name: String,
    val cwd: String,
    val command: String,
    val cols: Int = 100,
    val rows: Int = 30
)

@Serializable
data class DirItem(
    val name: String,
    val path: String
)

@Serializable
data class DirectoryResponse(
    val current: String,
    val parent: String,
    val dirs: List<DirItem> = emptyList()
)

@Serializable
data class WsControlMessage(
    val type: String,
    val data: String? = null,
    val cols: Int? = null,
    val rows: Int? = null
)

@Serializable
data class VpnStatus(
    val installed: Boolean = false,
    val connected: Boolean = false,
    val state: String = "Disconnected",
    val location: String = "",
    val ip: String = "",
    val protocol: String = "",
    val firewall: String = ""
)

@Serializable
data class ConnectVpnRequest(
    val location: String = "",
    val protocol: String = "",
    val port: String = ""
)

@Serializable
data class VpnLocation(
    val id: String,
    val name: String,
    val group: String = ""
)

@Serializable
data class VpnProtocol(
    val id: String,
    val name: String,
    val description: String = "",
    val ports: List<String> = emptyList()
)

@Serializable
data class VpnOptionsResponse(
    val locations: List<VpnLocation> = emptyList(),
    val protocols: List<VpnProtocol> = emptyList()
)

