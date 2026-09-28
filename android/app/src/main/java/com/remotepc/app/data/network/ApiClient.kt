package com.remotepc.app.data.network

import com.remotepc.app.data.model.CreateSessionRequest
import com.remotepc.app.data.model.DirectoryResponse
import com.remotepc.app.data.model.Session
import com.remotepc.app.data.model.VpnStatus
import com.remotepc.app.data.model.ConnectVpnRequest
import com.remotepc.app.data.model.VpnOptionsResponse
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import java.io.IOException
import java.util.concurrent.TimeUnit

class ApiClient(private val config: AppConfig) {
    private val client = OkHttpClient.Builder()
        .connectTimeout(5, TimeUnit.SECONDS)
        .readTimeout(10, TimeUnit.SECONDS)
        .build()

    private val json = Json {
        ignoreUnknownKeys = true
        isLenient = true
    }

    private fun newRequestBuilder(path: String): Request.Builder {
        val builder = Request.Builder()
            .url("${config.baseUrl}$path")
        if (config.token.isNotEmpty()) {
            builder.addHeader("Authorization", "Bearer ${config.token}")
        }
        return builder
    }

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

    suspend fun listDirectories(path: String = ""): DirectoryResponse = withContext(Dispatchers.IO) {
        val endpoint = if (path.isEmpty()) "/api/fs/directories" else "/api/fs/directories?path=${java.net.URLEncoder.encode(path, "UTF-8")}"
        val request = newRequestBuilder(endpoint).get().build()
        client.newCall(request).execute().use { response ->
            if (!response.isSuccessful) throw IOException("Failed to list dirs: ${response.code}")
            val body = response.body?.string() ?: "{}"
            json.decodeFromString(body)
        }
    }

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
        val bodyStr = json.encodeToString(ConnectVpnRequest.serializer(), req)
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
