package com.remotepc.app.ui.terminal

import android.annotation.SuppressLint
import android.util.Base64
import android.webkit.JavascriptInterface
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.automirrored.filled.Send
import androidx.compose.material.icons.filled.ClearAll
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.ZoomIn
import androidx.compose.material.icons.filled.ZoomOut
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import com.remotepc.app.data.network.AppConfig
import com.remotepc.app.data.network.TerminalEvent
import com.remotepc.app.data.network.TerminalWebSocket
import com.remotepc.app.ui.components.ClaudeShortcutBar
import com.remotepc.app.ui.theme.TerminalAccent
import com.remotepc.app.ui.theme.TerminalGreen
import com.remotepc.app.ui.theme.TerminalRed
import com.remotepc.app.ui.theme.TerminalText
import com.remotepc.app.ui.theme.TerminalTextMuted
import kotlinx.coroutines.launch

class TerminalBridge(
    private val onInputReceived: (String) -> Unit,
    private val onResizeReceived: (Int, Int) -> Unit,
    private val onTerminalClickedReceived: () -> Unit
) {
    @JavascriptInterface
    fun onInput(data: String) {
        onInputReceived(data)
    }

    @JavascriptInterface
    fun onResize(cols: Int, rows: Int) {
        onResizeReceived(cols, rows)
    }

    @JavascriptInterface
    fun onTerminalClicked() {
        onTerminalClickedReceived()
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@SuppressLint("SetJavaScriptEnabled")
@Composable
fun TerminalScreen(
    sessionId: String,
    sessionName: String,
    onBack: () -> Unit
) {
    val context = LocalContext.current
    val config = remember { AppConfig(context) }
    val scope = rememberCoroutineScope()
    val ws = remember { TerminalWebSocket(config, sessionId, scope) }
    val focusRequester = remember { FocusRequester() }
    val keyboardController = LocalSoftwareKeyboardController.current

    var connectionStatus by remember { mutableStateOf("Connecting...") }
    var inputText by remember { mutableStateOf("") }
    var fontSize by remember { mutableIntStateOf(10) }
    var webViewRef by remember { mutableStateOf<WebView?>(null) }
    var isHtmlReady by remember { mutableStateOf(false) }

    DisposableEffect(sessionId) {
        ws.connect()
        onDispose {
            ws.disconnect()
        }
    }

    LaunchedEffect(sessionId) {
        ws.events.collect { event ->
            when (event) {
                is TerminalEvent.Connected -> {
                    connectionStatus = "Connected"
                }
                is TerminalEvent.Disconnected -> {
                    connectionStatus = "Disconnected (${event.reason})"
                }
                is TerminalEvent.Error -> {
                    connectionStatus = "Error: ${event.throwable.localizedMessage}"
                }
                is TerminalEvent.OutputBytes -> {
                    val base64 = Base64.encodeToString(event.bytes, Base64.NO_WRAP)
                    webViewRef?.post {
                        webViewRef?.evaluateJavascript("writeRawBase64('$base64');", null)
                    }
                }
            }
        }
    }

    fun submitInput(text: String) {
        if (text.isNotEmpty()) {
            ws.sendInput(text + "\r")
            inputText = ""
        } else {
            ws.sendInput("\r")
        }
        webViewRef?.post {
            webViewRef?.evaluateJavascript("window.scrollToBottom();", null)
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Column {
                        Text(
                            text = sessionName.ifEmpty { "Terminal" },
                            style = MaterialTheme.typography.titleMedium,
                            color = TerminalText
                        )
                        Text(
                            text = connectionStatus,
                            fontSize = 11.sp,
                            color = when {
                                connectionStatus == "Connected" -> TerminalGreen
                                connectionStatus.startsWith("Error") -> TerminalRed
                                else -> TerminalTextMuted
                            }
                        )
                    }
                },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(
                            imageVector = Icons.AutoMirrored.Filled.ArrowBack,
                            contentDescription = "Back",
                            tint = TerminalText
                        )
                    }
                },
                actions = {
                    IconButton(onClick = {
                        webViewRef?.evaluateJavascript("clearTerminal();", null)
                    }) {
                        Icon(Icons.Default.ClearAll, contentDescription = "Clear", tint = TerminalTextMuted)
                    }
                    IconButton(onClick = {
                        if (fontSize > 8) {
                            fontSize -= 1
                            webViewRef?.evaluateJavascript("setFontSize($fontSize);", null)
                        }
                    }) {
                        Icon(Icons.Default.ZoomOut, contentDescription = "Zoom Out", tint = TerminalTextMuted)
                    }
                    IconButton(onClick = {
                        if (fontSize < 18) {
                            fontSize += 1
                            webViewRef?.evaluateJavascript("setFontSize($fontSize);", null)
                        }
                    }) {
                        Icon(Icons.Default.ZoomIn, contentDescription = "Zoom In", tint = TerminalTextMuted)
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = Color(0xFF161B22))
            )
        },
        bottomBar = {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .imePadding()
            ) {
                // Sleek Claude Shortcut toolbar
                ClaudeShortcutBar(
                    onSendKey = {
                        ws.sendInput(it)
                        webViewRef?.post {
                            webViewRef?.evaluateJavascript("window.scrollToBottom();", null)
                        }
                    },
                    onSendRaw = {
                        ws.sendRaw(it)
                        webViewRef?.post {
                            webViewRef?.evaluateJavascript("window.scrollToBottom();", null)
                        }
                    }
                )

                // Unified Prompt Input field & Send button
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .background(Color(0xFF161B22))
                        .padding(horizontal = 8.dp, vertical = 4.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    OutlinedTextField(
                        value = inputText,
                        onValueChange = { inputText = it },
                        modifier = Modifier
                            .weight(1f)
                            .focusRequester(focusRequester),
                        placeholder = {
                            Text("Type prompt or command...", fontSize = 12.sp, color = TerminalTextMuted)
                        },
                        trailingIcon = {
                            if (inputText.isNotEmpty()) {
                                IconButton(
                                    onClick = { inputText = "" },
                                    modifier = Modifier.size(24.dp)
                                ) {
                                    Icon(
                                        imageVector = Icons.Default.Close,
                                        contentDescription = "Clear",
                                        tint = TerminalTextMuted
                                    )
                                }
                            }
                        },
                        colors = OutlinedTextFieldDefaults.colors(
                            focusedTextColor = TerminalText,
                            unfocusedTextColor = TerminalText,
                            focusedBorderColor = TerminalAccent,
                            unfocusedBorderColor = Color(0xFF30363D),
                            focusedContainerColor = Color(0xFF0D1117),
                            unfocusedContainerColor = Color(0xFF0D1117)
                        ),
                        singleLine = true,
                        keyboardOptions = KeyboardOptions.Default.copy(imeAction = ImeAction.Send),
                        keyboardActions = KeyboardActions(onSend = { submitInput(inputText) }),
                        textStyle = MaterialTheme.typography.bodyMedium.copy(
                            fontFamily = FontFamily.Monospace,
                            fontSize = 12.sp
                        )
                    )

                    Spacer(modifier = Modifier.width(6.dp))

                    IconButton(
                        onClick = { submitInput(inputText) }
                    ) {
                        Icon(
                            imageVector = Icons.AutoMirrored.Filled.Send,
                            contentDescription = if (inputText.isNotBlank()) "Send Command" else "Send Enter",
                            tint = if (inputText.isNotBlank()) TerminalAccent else TerminalGreen
                        )
                    }
                }
            }
        },
        containerColor = Color(0xFF0D1117)
    ) { padding ->
        Box(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .background(Color(0xFF0D1117))
        ) {
            AndroidView(
                modifier = Modifier.fillMaxSize(),
                factory = { ctx ->
                    WebView(ctx).apply {
                        layoutParams = android.view.ViewGroup.LayoutParams(
                            android.view.ViewGroup.LayoutParams.MATCH_PARENT,
                            android.view.ViewGroup.LayoutParams.MATCH_PARENT
                        )
                        setBackgroundColor(0xFF0D1117.toInt())
                        setLayerType(android.view.View.LAYER_TYPE_HARDWARE, null)
                        overScrollMode = android.view.View.OVER_SCROLL_NEVER
                        isVerticalScrollBarEnabled = false
                        isHorizontalScrollBarEnabled = false

                        settings.javaScriptEnabled = true
                        settings.domStorageEnabled = true
                        settings.allowFileAccess = true
                        settings.allowContentAccess = true
                        settings.allowFileAccessFromFileURLs = true
                        settings.allowUniversalAccessFromFileURLs = true
                        settings.cacheMode = WebSettings.LOAD_NO_CACHE

                        webChromeClient = object : android.webkit.WebChromeClient() {
                            override fun onConsoleMessage(msg: android.webkit.ConsoleMessage?): Boolean {
                                android.util.Log.e("TerminalWeb", "${msg?.message()} [line ${msg?.lineNumber()} of ${msg?.sourceId()}]")
                                return true
                            }
                        }

                        val bridge = TerminalBridge(
                            onInputReceived = { data -> ws.sendInput(data) },
                            onResizeReceived = { cols, rows -> ws.sendResize(cols, rows) },
                            onTerminalClickedReceived = {
                                scope.launch {
                                    focusRequester.requestFocus()
                                    keyboardController?.show()
                                    postDelayed({
                                        evaluateJavascript("window.scrollToBottom();", null)
                                    }, 250)
                                }
                            }
                        )
                        addJavascriptInterface(bridge, "AndroidBridge")

                        webViewClient = object : WebViewClient() {
                            override fun onPageFinished(view: WebView?, url: String?) {
                                android.util.Log.d("TerminalWeb", "Page finished loading: $url")
                                isHtmlReady = true
                            }
                        }

                        loadUrl("file:///android_asset/terminal/terminal.html")
                        webViewRef = this
                    }
                }
            )
        }
    }
}
