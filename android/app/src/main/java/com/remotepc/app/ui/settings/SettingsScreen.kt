package com.remotepc.app.ui.settings

import android.content.ClipboardManager
import android.content.Context
import android.net.Uri
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.ContentPaste
import androidx.compose.material.icons.filled.QrCodeScanner
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.google.mlkit.vision.barcode.common.Barcode
import com.google.mlkit.vision.codescanner.GmsBarcodeScannerOptions
import com.google.mlkit.vision.codescanner.GmsBarcodeScanning
import com.remotepc.app.data.network.ApiClient
import com.remotepc.app.data.network.AppConfig
import com.remotepc.app.ui.theme.TerminalAccent
import com.remotepc.app.ui.theme.TerminalBackground
import com.remotepc.app.ui.theme.TerminalGreen
import com.remotepc.app.ui.theme.TerminalRed
import com.remotepc.app.ui.theme.TerminalSurface
import com.remotepc.app.ui.theme.TerminalSurfaceVariant
import com.remotepc.app.ui.theme.TerminalText
import com.remotepc.app.ui.theme.TerminalTextMuted
import kotlinx.coroutines.launch
import org.json.JSONObject

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsScreen(
    onBack: () -> Unit
) {
    val context = LocalContext.current
    val config = remember { AppConfig(context) }

    var host by remember { mutableStateOf(config.host) }
    var port by remember { mutableStateOf(config.port) }
    var token by remember { mutableStateOf(config.token) }

    var testStatus by remember { mutableStateOf<String?>(null) }
    var isTesting by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()

    val gmsScanner = remember {
        val options = GmsBarcodeScannerOptions.Builder()
            .setBarcodeFormats(Barcode.FORMAT_QR_CODE)
            .enableAutoZoom()
            .build()
        GmsBarcodeScanning.getClient(context, options)
    }

    fun saveConfig() {
        config.host = host
        config.port = port
        config.token = token
    }

    fun parseAndApply(payload: String): Boolean {
        val trimmed = payload.trim()
        try {
            // Case 1: orbit:// or http:// or https:// URI scheme
            if (trimmed.startsWith("orbit://", ignoreCase = true) ||
                trimmed.startsWith("http://", ignoreCase = true) ||
                trimmed.startsWith("https://", ignoreCase = true)
            ) {
                val uri = Uri.parse(trimmed)
                val scannedHost = uri.host
                val scannedPort = if (uri.port != -1) uri.port.toString() else "8080"
                val scannedToken = uri.getQueryParameter("token") ?: ""

                if (!scannedHost.isNullOrBlank()) {
                    host = scannedHost
                    port = scannedPort
                    token = scannedToken
                    saveConfig()
                    return true
                }
            }

            // Case 2: JSON configuration { "host": "...", "port": "...", "token": "..." }
            if (trimmed.startsWith("{")) {
                val json = JSONObject(trimmed)
                val scannedHost = json.optString("host", "")
                val scannedPort = json.optString("port", "8080")
                val scannedToken = json.optString("token", "")

                if (scannedHost.isNotBlank()) {
                    host = scannedHost
                    port = scannedPort
                    token = scannedToken
                    saveConfig()
                    return true
                }
            }
        } catch (_: Exception) {
            return false
        }
        return false
    }

    fun launchQrScanner() {
        gmsScanner.startScan()
            .addOnSuccessListener { barcode ->
                val raw = barcode.rawValue ?: ""
                if (parseAndApply(raw)) {
                    isTesting = true
                    testStatus = "Testing connection..."
                    scope.launch {
                        val client = ApiClient(config)
                        val ok = client.checkHealth()
                        testStatus = if (ok) "✓ Connection Successful to Orbit PC!" else "✗ Config applied, but cannot reach host. Check Wi-Fi or Tailscale."
                        isTesting = false
                    }
                } else {
                    testStatus = "✗ Unrecognized QR code format: $raw"
                }
            }
            .addOnFailureListener { e ->
                testStatus = "Scan cancelled or failed: ${e.message}"
            }
    }

    fun pasteFromClipboard() {
        val clipboard = context.getSystemService(Context.CLIPBOARD_SERVICE) as? ClipboardManager
        val clip = clipboard?.primaryClip?.getItemAt(0)?.text?.toString()?.trim()
        if (!clip.isNullOrBlank()) {
            if (parseAndApply(clip)) {
                isTesting = true
                testStatus = "Testing connection..."
                scope.launch {
                    val client = ApiClient(config)
                    val ok = client.checkHealth()
                    testStatus = if (ok) "✓ Connection Successful to Orbit PC!" else "✗ Config applied, but cannot reach host."
                    isTesting = false
                }
            } else {
                testStatus = "✗ Clipboard does not contain an Orbit pairing URI or JSON config."
            }
        } else {
            testStatus = "✗ Clipboard is empty."
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Connection Settings", color = TerminalText) },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back", tint = TerminalText)
                    }
                },
                actions = {
                    IconButton(onClick = { launchQrScanner() }) {
                        Icon(Icons.Default.QrCodeScanner, contentDescription = "Scan QR Code", tint = TerminalAccent)
                    }
                    IconButton(onClick = {
                        saveConfig()
                        onBack()
                    }) {
                        Icon(Icons.Default.Check, contentDescription = "Save", tint = TerminalGreen)
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = TerminalSurface)
            )
        },
        containerColor = TerminalBackground
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .background(TerminalBackground)
                .padding(16.dp)
        ) {
            // Fast Pairing Card (QR & Clipboard)
            Card(
                colors = CardDefaults.cardColors(containerColor = TerminalSurfaceVariant),
                shape = RoundedCornerShape(12.dp),
                modifier = Modifier.fillMaxWidth()
            ) {
                Column(modifier = Modifier.padding(14.dp)) {
                    Text(
                        text = "Instant 1-Click Pairing",
                        style = MaterialTheme.typography.titleSmall,
                        color = TerminalText
                    )
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(
                        text = "Scan the QR code displayed on your PC Orbit Cockpit or paste config.",
                        style = MaterialTheme.typography.bodySmall,
                        color = TerminalTextMuted
                    )
                    Spacer(modifier = Modifier.height(12.dp))
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        Button(
                            onClick = { launchQrScanner() },
                            modifier = Modifier.weight(1f),
                            colors = ButtonDefaults.buttonColors(containerColor = TerminalAccent)
                        ) {
                            Icon(Icons.Default.QrCodeScanner, contentDescription = null, modifier = Modifier.height(18.dp))
                            Spacer(modifier = Modifier.width(6.dp))
                            Text("Scan QR", color = TerminalSurface)
                        }
                        OutlinedButton(
                            onClick = { pasteFromClipboard() },
                            modifier = Modifier.weight(1f)
                        ) {
                            Icon(Icons.Default.ContentPaste, contentDescription = null, modifier = Modifier.height(18.dp))
                            Spacer(modifier = Modifier.width(6.dp))
                            Text("Paste")
                        }
                    }
                }
            }

            Spacer(modifier = Modifier.height(20.dp))

            Text(
                text = "Manual Configuration",
                style = MaterialTheme.typography.labelMedium,
                color = TerminalTextMuted
            )

            Spacer(modifier = Modifier.height(10.dp))

            OutlinedTextField(
                value = host,
                onValueChange = { host = it },
                label = { Text("PC IP Address (LAN or Tailscale)") },
                placeholder = { Text("e.g. 192.168.1.50 or 100.x.y.z") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            Spacer(modifier = Modifier.height(12.dp))

            OutlinedTextField(
                value = port,
                onValueChange = { port = it },
                label = { Text("Port") },
                placeholder = { Text("8080") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            Spacer(modifier = Modifier.height(12.dp))

            OutlinedTextField(
                value = token,
                onValueChange = { token = it },
                label = { Text("Auth Token") },
                placeholder = { Text("Token shown in server console / Cockpit") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            Spacer(modifier = Modifier.height(16.dp))

            OutlinedButton(
                onClick = {
                    saveConfig()
                    isTesting = true
                    testStatus = "Testing connection..."
                    scope.launch {
                        val client = ApiClient(config)
                        val ok = client.checkHealth()
                        testStatus = if (ok) "✓ Connection Successful!" else "✗ Connection Failed. Check IP and Port."
                        isTesting = false
                    }
                },
                modifier = Modifier.fillMaxWidth(),
                enabled = !isTesting
            ) {
                Text(if (isTesting) "Connecting..." else "Test Connection")
            }

            if (testStatus != null) {
                Spacer(modifier = Modifier.height(8.dp))
                Text(
                    text = testStatus ?: "",
                    color = if (testStatus?.startsWith("✓") == true) TerminalGreen else TerminalRed,
                    fontSize = 13.sp,
                    modifier = Modifier.padding(horizontal = 4.dp)
                )
            }

            Spacer(modifier = Modifier.weight(1f))

            Button(
                onClick = {
                    saveConfig()
                    onBack()
                },
                modifier = Modifier.fillMaxWidth(),
                colors = ButtonDefaults.buttonColors(containerColor = TerminalAccent)
            ) {
                Text("Save and Return", color = TerminalSurface)
            }
        }
    }
}
