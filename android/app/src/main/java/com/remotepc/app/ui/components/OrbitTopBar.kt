package com.remotepc.app.ui.components

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Security
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.remotepc.app.data.model.VpnStatus
import com.remotepc.app.ui.theme.*

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun OrbitTopBar(
    title: String,
    subtitle: String? = null,
    isConnected: Boolean,
    vpnStatus: VpnStatus? = null,
    onRefresh: () -> Unit,
    onOpenSettings: () -> Unit
) {
    TopAppBar(
        title = {
            Column {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        text = title,
                        color = TerminalText,
                        fontWeight = FontWeight.ExtraBold,
                        fontSize = 19.sp
                    )
                    Spacer(modifier = Modifier.width(8.dp))
                    // Connection Status Pill
                    Surface(
                        color = if (isConnected) TerminalGreen.copy(alpha = 0.15f) else TerminalRed.copy(alpha = 0.15f),
                        shape = RoundedCornerShape(12.dp)
                    ) {
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            modifier = Modifier.padding(horizontal = 8.dp, vertical = 2.dp)
                        ) {
                            Box(
                                modifier = Modifier
                                    .size(7.dp)
                                    .clip(CircleShape)
                                    .background(if (isConnected) TerminalGreen else TerminalRed)
                            )
                            Spacer(modifier = Modifier.width(5.dp))
                            Text(
                                text = if (isConnected) "ONLINE" else "OFFLINE",
                                color = if (isConnected) TerminalGreen else TerminalRed,
                                fontSize = 10.sp,
                                fontWeight = FontWeight.Bold
                            )
                        }
                    }

                    // VPN Shield Pill if installed
                    if (vpnStatus != null && vpnStatus.installed) {
                        Spacer(modifier = Modifier.width(6.dp))
                        val isVpnOn = vpnStatus.connected
                        Surface(
                            color = if (isVpnOn) TerminalAccent.copy(alpha = 0.15f) else TerminalTextDim.copy(alpha = 0.15f),
                            shape = RoundedCornerShape(12.dp)
                        ) {
                            Row(
                                verticalAlignment = Alignment.CenterVertically,
                                modifier = Modifier.padding(horizontal = 7.dp, vertical = 2.dp)
                            ) {
                                Icon(
                                    imageVector = Icons.Default.Security,
                                    contentDescription = null,
                                    tint = if (isVpnOn) TerminalAccent else TerminalTextDim,
                                    modifier = Modifier.size(11.dp)
                                )
                                Spacer(modifier = Modifier.width(3.dp))
                                Text(
                                    text = if (isVpnOn) "VPN ON" else "VPN OFF",
                                    color = if (isVpnOn) TerminalAccent else TerminalTextDim,
                                    fontSize = 10.sp,
                                    fontWeight = FontWeight.Bold
                                )
                            }
                        }
                    }
                }
                if (subtitle != null) {
                    Text(
                        text = subtitle,
                        color = TerminalTextMuted,
                        fontSize = 11.sp
                    )
                }
            }
        },
        actions = {
            IconButton(onClick = onRefresh) {
                Icon(
                    imageVector = Icons.Default.Refresh,
                    contentDescription = "Refresh",
                    tint = TerminalTextMuted
                )
            }
            IconButton(onClick = onOpenSettings) {
                Icon(
                    imageVector = Icons.Default.Settings,
                    contentDescription = "Settings",
                    tint = TerminalTextMuted
                )
            }
        },
        colors = TopAppBarDefaults.topAppBarColors(
            containerColor = TerminalBackground
        )
    )
}
