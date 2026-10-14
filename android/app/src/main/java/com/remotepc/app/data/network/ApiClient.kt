package com.remotepc.app.data.network

import com.remotepc.app.data.model.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import java.io.IOException
import java.net.URLEncoder
import java.util.concurrent.TimeUnit

class ApiClient(private val config: AppConfig) {
    private val client = OkHttpClient.Builder()
        .connectTimeout(5, TimeUnit.SECONDS)
        .readTimeout(10, TimeUnit.SECONDS)
        .build()

    private val json = Json {
        ignoreUnknownKeys = true
        isLenient = true
        coerceInputValues = true
    }

    private fun newRequestBuilder(path: String): Request.Builder {
        val builder = Request.Builder()
            .url("${config.baseUrl}$path")
        if (config.token.isNotEmpty()) {
            builder.addHeader("Authorization", "Bearer ${config.token}")
        }
        return builder
    }

    // Health
    suspend fun checkHealth(): Boolean = withContext(Dispatchers.IO) {
        try {
            val request = newRequestBuilder("/api/health").get().build()
            client.newCall(request).execute().use { response ->
                response.isSuccessful
            }
        } catch (e: Exception) {
            false
        }
    }

    // Server Info
    suspend fun getServerInfo(): ServerInfoResponse = withContext(Dispatchers.IO) {
        val request = newRequestBuilder("/api/server-info").get().build()
        client.newCall(request).execute().use { response ->
            if (!response.isSuccessful) throw IOException("Failed to get server info: ${response.code}")
            val body = response.body?.string() ?: "{}"
            json.decodeFromString(body)
        }
    }

    // Sessions
    suspend fun listSessions(): List<Session> = withContext(Dispatchers.IO) {
        val request = newRequestBuilder("/api/sessions").get().build()
        client.newCall(request).execute().use { response ->
            if (!response.isSuccessful) throw IOException("Failed to list sessions: ${response.code}")
            val body = response.body?.string() ?: "[]"
            json.decodeFromString(body)
        }
    }

    suspend fun createSession(req: CreateSessionRequest): Session = withContext(Dispatchers.IO) {
        val mediaType = "application/json".toMediaType()
        val body = json.encodeToString(req).toRequestBody(mediaType)
        val request = newRequestBuilder("/api/sessions").post(body).build()
        client.newCall(request).execute().use { response ->
            if (!response.isSuccessful) throw IOException("Failed to create session: ${response.code} ${response.body?.string()}")
            val responseBody = response.body?.string() ?: throw IOException("Empty response")
            json.decodeFromString(responseBody)
        }
    }

    suspend fun deleteSession(sessionId: String): Boolean = withContext(Dispatchers.IO) {
        val request = newRequestBuilder("/api/sessions/$sessionId").delete().build()
        client.newCall(request).execute().use { response ->
            response.isSuccessful
        }
    }

    suspend fun getSessionHistory(sessionId: String): String = withContext(Dispatchers.IO) {
        try {
            val request = newRequestBuilder("/api/sessions/$sessionId/history").get().build()
            client.newCall(request).execute().use { response ->
                if (!response.isSuccessful) return@withContext ""
                val body = response.body?.string() ?: return@withContext ""
                val parsed = json.parseToJsonElement(body).jsonObject
                parsed["snippet"]?.jsonPrimitive?.content ?: ""
            }
        } catch (e: Exception) {
            ""
        }
    }

    suspend fun renameSession(sessionId: String, name: String): Boolean = withContext(Dispatchers.IO) {
        val bodyStr = """{"name":"$name"}"""
        val body = bodyStr.toRequestBody("application/json".toMediaType())
        val request = newRequestBuilder("/api/sessions/$sessionId").patch(body).build()
        client.newCall(request).execute().use { response ->
            response.isSuccessful
        }
    }

    // Bubbles & Worktrees
    suspend fun listBubbles(scan: Boolean = true): List<Bubble> = withContext(Dispatchers.IO) {
        val path = if (scan) "/api/bubbles?scan=true" else "/api/bubbles"
        val request = newRequestBuilder(path).get().build()
        client.newCall(request).execute().use { response ->
            if (!response.isSuccessful) throw IOException("Failed to list bubbles: ${response.code}")
            val body = response.body?.string() ?: "[]"
            json.decodeFromString(body)
        }
    }

    suspend fun createBubble(req: CreateBubbleRequest): Bubble = withContext(Dispatchers.IO) {
        val mediaType = "application/json".toMediaType()
        val body = json.encodeToString(req).toRequestBody(mediaType)
        val request = newRequestBuilder("/api/bubbles").post(body).build()
        client.newCall(request).execute().use { response ->
            if (!response.isSuccessful) throw IOException("Failed to create bubble: ${response.code} ${response.body?.string()}")
            val responseBody = response.body?.string() ?: throw IOException("Empty response")
            json.decodeFromString(responseBody)
        }
    }

    suspend fun deleteBubble(bubbleId: String): Boolean = withContext(Dispatchers.IO) {
        val request = newRequestBuilder("/api/bubbles/$bubbleId").delete().build()
        client.newCall(request).execute().use { response ->
            response.isSuccessful
        }
    }

    suspend fun listWorkspaces(): List<WorkspaceInfo> = withContext(Dispatchers.IO) {
        val request = newRequestBuilder("/api/workspaces").get().build()
        client.newCall(request).execute().use { response ->
            if (!response.isSuccessful) throw IOException("Failed to list workspaces: ${response.code}")
            val body = response.body?.string() ?: "[]"
            json.decodeFromString(body)
        }
    }

    suspend fun getWorkspaceBranches(workspaceId: String): List<BranchInfo> = withContext(Dispatchers.IO) {
        val encoded = URLEncoder.encode(workspaceId, "UTF-8")
        val request = newRequestBuilder("/api/workspaces/$encoded/branches").get().build()
        client.newCall(request).execute().use { response ->
            if (!response.isSuccessful) throw IOException("Failed to list branches: ${response.code}")
            val body = response.body?.string() ?: "[]"
            json.decodeFromString(body)
        }
    }

    suspend fun listSqlDumps(): List<SqlDumpInfo> = withContext(Dispatchers.IO) {
        val request = newRequestBuilder("/api/system/sql-dumps").get().build()
        client.newCall(request).execute().use { response ->
            if (!response.isSuccessful) throw IOException("Failed to list SQL dumps: ${response.code}")
            val body = response.body?.string() ?: "[]"
            json.decodeFromString(body)
        }
    }

    suspend fun linkWorkspaceRepo(workspaceId: String, repoUrl: String): Boolean = withContext(Dispatchers.IO) {
        val encoded = URLEncoder.encode(workspaceId, "UTF-8")
        val bodyStr = """{"repoUrl":"$repoUrl"}"""
        val body = bodyStr.toRequestBody("application/json".toMediaType())
        val request = newRequestBuilder("/api/workspaces/$encoded/link-repo").post(body).build()
        client.newCall(request).execute().use { response ->
            response.isSuccessful
        }
    }

    suspend fun startDevServer(bubbleId: String): Boolean = withContext(Dispatchers.IO) {
        val request = newRequestBuilder("/api/bubbles/$bubbleId/dev/start").post("".toRequestBody(null)).build()
        client.newCall(request).execute().use { response ->
            response.isSuccessful
        }
    }

    suspend fun stopDevServer(bubbleId: String): Boolean = withContext(Dispatchers.IO) {
        val request = newRequestBuilder("/api/bubbles/$bubbleId/dev/stop").post("".toRequestBody(null)).build()
        client.newCall(request).execute().use { response ->
            response.isSuccessful
        }
    }

    suspend fun getGitStatus(bubbleId: String): GitStatusResponse = withContext(Dispatchers.IO) {
        val request = newRequestBuilder("/api/bubbles/$bubbleId/git/status").get().build()
        client.newCall(request).execute().use { response ->
            if (!response.isSuccessful) throw IOException("Failed to get git status: ${response.code}")
            val body = response.body?.string() ?: "{}"
            json.decodeFromString(body)
        }
    }

    // Calendar & Daily Notes
    suspend fun getDayNote(date: String = ""): DayNote = withContext(Dispatchers.IO) {
        val endpoint = if (date.isEmpty()) "/api/calendar/note" else "/api/calendar/note?date=${URLEncoder.encode(date, "UTF-8")}"
        val request = newRequestBuilder(endpoint).get().build()
        client.newCall(request).execute().use { response ->
            if (!response.isSuccessful) throw IOException("Failed to get day note: ${response.code}")
            val body = response.body?.string() ?: "{}"
            json.decodeFromString(body)
        }
    }

    suspend fun saveDayNote(date: String, content: String): Boolean = withContext(Dispatchers.IO) {
        val req = SaveNoteRequest(date = date, content = content)
        val body = json.encodeToString(req).toRequestBody("application/json".toMediaType())
        val request = newRequestBuilder("/api/calendar/note").put(body).build()
        client.newCall(request).execute().use { response ->
            response.isSuccessful
        }
    }

    suspend fun addDailyTodo(text: String, date: String = "", checked: Boolean = false): DayNote = withContext(Dispatchers.IO) {
        val req = AddTodoRequest(text = text, date = date, checked = checked)
        val body = json.encodeToString(req).toRequestBody("application/json".toMediaType())
        val request = newRequestBuilder("/api/calendar/todo").post(body).build()
        client.newCall(request).execute().use { response ->
            if (!response.isSuccessful) throw IOException("Failed to add todo: ${response.code}")
            val resBody = response.body?.string() ?: "{}"
            json.decodeFromString(resBody)
        }
    }

    suspend fun appendDailyNote(text: String, date: String = ""): DayNote = withContext(Dispatchers.IO) {
        val req = AppendNoteRequest(text = text, date = date)
        val body = json.encodeToString(req).toRequestBody("application/json".toMediaType())
        val request = newRequestBuilder("/api/calendar/note/append").post(body).build()
        client.newCall(request).execute().use { response ->
            if (!response.isSuccessful) throw IOException("Failed to append note: ${response.code}")
            val resBody = response.body?.string() ?: "{}"
            json.decodeFromString(resBody)
        }
    }

    suspend fun getStandup(): StandupSummary = withContext(Dispatchers.IO) {
        val request = newRequestBuilder("/api/calendar/standup").get().build()
        client.newCall(request).execute().use { response ->
            if (!response.isSuccessful) throw IOException("Failed to get standup: ${response.code}")
            val body = response.body?.string() ?: "{}"
            json.decodeFromString(body)
        }
    }

    // System Monitoring & Stats
    suspend fun getSystemStats(withTop: Boolean = true): SystemStats = withContext(Dispatchers.IO) {
        val endpoint = if (withTop) "/api/system/stats?top=true" else "/api/system/stats"
        val request = newRequestBuilder(endpoint).get().build()
        client.newCall(request).execute().use { response ->
            if (!response.isSuccessful) throw IOException("Failed to get system stats: ${response.code}")
            val body = response.body?.string() ?: "{}"
            json.decodeFromString(body)
        }
    }

    suspend fun killProcess(pid: Long): Boolean = withContext(Dispatchers.IO) {
        val bodyStr = """{"pid":$pid}"""
        val body = bodyStr.toRequestBody("application/json".toMediaType())
        val request = newRequestBuilder("/api/system/process/kill").post(body).build()
        client.newCall(request).execute().use { response ->
            response.isSuccessful
        }
    }

    // File System Directory Browser
    suspend fun listDirectories(path: String = ""): DirectoryResponse = withContext(Dispatchers.IO) {
        val endpoint = if (path.isEmpty()) "/api/fs/directories" else "/api/fs/directories?path=${URLEncoder.encode(path, "UTF-8")}"
        val request = newRequestBuilder(endpoint).get().build()
        client.newCall(request).execute().use { response ->
            if (!response.isSuccessful) throw IOException("Failed to list dirs: ${response.code}")
            val body = response.body?.string() ?: "{}"
            json.decodeFromString(body)
        }
    }

    // VPN Controls
    suspend fun getVpnStatus(): VpnStatus = withContext(Dispatchers.IO) {
        val request = newRequestBuilder("/api/vpn/status").get().build()
        client.newCall(request).execute().use { response ->
            if (!response.isSuccessful) throw IOException("Failed to get VPN status: ${response.code}")
            val body = response.body?.string() ?: "{}"
            json.decodeFromString(body)
        }
    }

    suspend fun getVpnOptions(): VpnOptionsResponse = withContext(Dispatchers.IO) {
        val request = newRequestBuilder("/api/vpn/options").get().build()
        client.newCall(request).execute().use { response ->
            if (!response.isSuccessful) throw IOException("Failed to get VPN options: ${response.code}")
            val body = response.body?.string() ?: "{}"
            json.decodeFromString(body)
        }
    }

    suspend fun connectVpn(location: String = "", protocol: String = "", port: String = ""): VpnStatus = withContext(Dispatchers.IO) {
        val req = ConnectVpnRequest(location = location, protocol = protocol, port = port)
        val bodyStr = json.encodeToString(req)
        val body = bodyStr.toRequestBody("application/json".toMediaType())
        val request = newRequestBuilder("/api/vpn/connect").post(body).build()
        client.newCall(request).execute().use { response ->
            if (!response.isSuccessful) throw IOException("Failed to connect VPN: ${response.code}")
            val resBody = response.body?.string() ?: "{}"
            json.decodeFromString(resBody)
        }
    }

    suspend fun disconnectVpn(): VpnStatus = withContext(Dispatchers.IO) {
        val body = "".toRequestBody(null)
        val request = newRequestBuilder("/api/vpn/disconnect").post(body).build()
        client.newCall(request).execute().use { response ->
            if (!response.isSuccessful) throw IOException("Failed to disconnect VPN: ${response.code}")
            val resBody = response.body?.string() ?: "{}"
            json.decodeFromString(resBody)
        }
    }
}
