package com.remotepc.app.ui.components

import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.Assignment
import androidx.compose.material.icons.filled.Dns
import androidx.compose.material.icons.filled.Layers
import androidx.compose.material.icons.filled.Terminal
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.remotepc.app.ui.theme.*

enum class OrbitTab(val title: String, val icon: ImageVector) {
    Bubbles("Tasks", Icons.Default.Layers),
    Sessions("Terminals", Icons.Default.Terminal),
    Daily("Daily", Icons.AutoMirrored.Filled.Assignment),
    Monitor("Monitor", Icons.Default.Dns)
}

@Composable
fun OrbitBottomBar(
    currentTab: OrbitTab,
    onTabSelected: (OrbitTab) -> Unit,
    sessionsCount: Int = 0,
    bubblesCount: Int = 0
) {
    NavigationBar(
        containerColor = TerminalSurface,
        tonalElevation = 8.dp
    ) {
        OrbitTab.values().forEach { tab ->
            val selected = currentTab == tab
            NavigationBarItem(
                selected = selected,
                onClick = { onTabSelected(tab) },
                icon = {
                    BadgedBox(badge = {
                        if (tab == OrbitTab.Sessions && sessionsCount > 0) {
                            Badge(
                                containerColor = TerminalAccent,
                                contentColor = TerminalBackground
                            ) {
                                Text("$sessionsCount", fontSize = 10.sp)
                            }
                        }
                    }) {
                        Icon(
                            imageVector = tab.icon,
                            contentDescription = tab.title,
                            modifier = Modifier.size(22.dp)
                        )
                    }
                },
                label = {
                    Text(
                        text = tab.title,
                        fontSize = 11.sp,
                        color = if (selected) TerminalAccent else TerminalTextMuted
                    )
                },
                colors = NavigationBarItemDefaults.colors(
                    selectedIconColor = TerminalAccent,
                    selectedTextColor = TerminalAccent,
                    unselectedIconColor = TerminalTextDim,
                    unselectedTextColor = TerminalTextDim,
                    indicatorColor = TerminalSurfaceVariant
                )
            )
        }
    }
}
