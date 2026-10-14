package com.remotepc.app.data.model

import kotlinx.serialization.Serializable

@Serializable
data class ProcessInfo(
    val pid: Long,
    val name: String,
    val ramMB: Double = 0.0
)

@Serializable
data class MemoryStats(
    val totalGB: Double = 0.0,
    val usedGB: Double = 0.0,
    val freeGB: Double = 0.0,
    val percent: Int = 0,
    val totalPageGB: Double = 0.0,
    val usedPageGB: Double = 0.0,
    val pagePercent: Int = 0
)

@Serializable
data class ServiceInfo(
    val id: String = "",
    val name: String = "",
    val port: Int = 0,
    val running: Boolean = false,
    val pingMs: Long = 0
)

@Serializable
data class SystemStats(
    val memory: MemoryStats = MemoryStats(),
    val services: List<ServiceInfo> = emptyList(),
    val topProcesses: List<ProcessInfo> = emptyList(),
    val checkedAt: String = ""
)

@Serializable
data class ServerInfoResponse(
    val version: String = "",
    val hostname: String = "",
    val port: String = "",
    val lanIps: List<String> = emptyList(),
    val uptime: String = "",
    val sessionCount: Int = 0,
    val clientCount: Int = 0,
    val vpn: VpnStatus? = null
)
