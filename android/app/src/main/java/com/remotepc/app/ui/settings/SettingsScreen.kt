package com.remotepc.app.ui.settings

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Check
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
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
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.remotepc.app.data.network.ApiClient
import com.remotepc.app.data.network.AppConfig
import com.remotepc.app.ui.theme.TerminalAccent
import com.remotepc.app.ui.theme.TerminalBackground
import com.remotepc.app.ui.theme.TerminalGreen
import com.remotepc.app.ui.theme.TerminalRed
import com.remotepc.app.ui.theme.TerminalSurface
import com.remotepc.app.ui.theme.TerminalText
import com.remotepc.app.ui.theme.TerminalTextMuted
import kotlinx.coroutines.launch

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

    fun saveConfig() {
        config.host = host
        config.port = port
        config.token = token
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
            Text(
                text = "Configure the PC Host IP and security token to connect over LAN or Tailscale.",
                style = MaterialTheme.typography.bodySmall,
                color = TerminalTextMuted
            )

            Spacer(modifier = Modifier.height(16.dp))

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
                placeholder = { Text("Token shown in server console") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            Spacer(modifier = Modifier.height(20.dp))

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
