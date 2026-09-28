package com.remotepc.app.ui.theme

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable

private val DarkColorScheme = darkColorScheme(
    primary = TerminalAccent,
    secondary = TerminalGreen,
    tertiary = TerminalAmber,
    background = TerminalBackground,
    surface = TerminalSurface,
    onPrimary = TerminalBackground,
    onSecondary = TerminalBackground,
    onBackground = TerminalText,
    onSurface = TerminalText,
)

@Composable
fun RemotePCTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = DarkColorScheme,
        content = content
    )
}
