package com.remotepc.app.data.network

import com.remotepc.app.data.model.WsControlMessage
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.launch
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import okio.ByteString
import java.util.concurrent.TimeUnit

sealed class TerminalEvent {
    data class OutputBytes(val bytes: ByteArray) : TerminalEvent() {
        override fun equals(other: Any?): Boolean {
            if (this === other) return true
            if (javaClass != other?.javaClass) return false
            other as OutputBytes
            return bytes.contentEquals(other.bytes)
        }
        override fun hashCode(): Int = bytes.contentHashCode()
    }
    data class Connected(val sessionId: String) : TerminalEvent()
    data class Disconnected(val reason: String) : TerminalEvent()
    data class Error(val throwable: Throwable) : TerminalEvent()
}

class TerminalWebSocket(
    private val config: AppConfig,
    private val sessionId: String,
    private val scope: CoroutineScope
) {
    private val client = OkHttpClient.Builder()
        .readTimeout(0, TimeUnit.MILLISECONDS)
        .pingInterval(10, TimeUnit.SECONDS)
        .build()

    private var webSocket: WebSocket? = null
    private val _events = MutableSharedFlow<TerminalEvent>(replay = 0, extraBufferCapacity = 64)
    val events: SharedFlow<TerminalEvent> = _events.asSharedFlow()

    private val json = Json { ignoreUnknownKeys = true }

    fun connect() {
        val url = buildString {
            append("${config.wsUrl}/ws/session/$sessionId")
            if (config.token.isNotEmpty()) {
                append("?token=${java.net.URLEncoder.encode(config.token, "UTF-8")}")
            }
        }

        val request = Request.Builder().url(url).build()

        webSocket = client.newWebSocket(request, object : WebSocketListener() {
            override fun onOpen(webSocket: WebSocket, response: Response) {
                scope.launch {
                    _events.emit(TerminalEvent.Connected(sessionId))
                }
            }

            override fun onMessage(webSocket: WebSocket, bytes: ByteString) {
                val data = bytes.toByteArray()
                scope.launch {
                    _events.emit(TerminalEvent.OutputBytes(data))
                }
            }

            override fun onMessage(webSocket: WebSocket, text: String) {
                val data = text.toByteArray(Charsets.UTF_8)
                scope.launch {
                    _events.emit(TerminalEvent.OutputBytes(data))
                }
            }

            override fun onClosing(webSocket: WebSocket, code: Int, reason: String) {
                scope.launch {
                    _events.emit(TerminalEvent.Disconnected(reason))
                }
            }

            override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                scope.launch {
                    _events.emit(TerminalEvent.Error(t))
                }
            }
        })
    }

    fun sendInput(text: String) {
        val msg = WsControlMessage(type = "input", data = text)
        webSocket?.send(json.encodeToString(msg))
    }

    fun sendRaw(bytes: ByteArray) {
        webSocket?.send(ByteString.of(*bytes))
    }

    fun sendResize(cols: Int, rows: Int) {
        val msg = WsControlMessage(type = "resize", cols = cols, rows = rows)
        webSocket?.send(json.encodeToString(msg))
    }

    fun disconnect() {
        webSocket?.close(1000, "User disconnected")
        webSocket = null
    }
}
