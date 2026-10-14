package com.remotepc.app.ui.components

import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.remotepc.app.ui.theme.*

@Composable
fun ClaudeShortcutBar(
    onSendKey: (String) -> Unit,
    onSendRaw: (ByteArray) -> Unit,
    modifier: Modifier = Modifier
) {
    Surface(
        modifier = modifier.fillMaxWidth(),
        color = TerminalSurface,
        tonalElevation = 4.dp
    ) {
        Column(modifier = Modifier.padding(horizontal = 6.dp, vertical = 4.dp)) {
            // Row 1: Special Navigation & Control Keys (Esc, Tab, Ctrl+C, Arrows, Enter)
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .horizontalScroll(rememberScrollState()),
                horizontalArrangement = Arrangement.spacedBy(4.dp)
            ) {
                CompactButton("ESC", TerminalText) { onSendRaw(byteArrayOf(0x1B)) }
                CompactButton("TAB", TerminalText) { onSendKey("\t") }
                CompactButton("CTRL+C", TerminalRed) { onSendRaw(byteArrayOf(0x03)) }
                CompactButton("CTRL+D", TerminalTextMuted) { onSendRaw(byteArrayOf(0x04)) }
                CompactButton("CTRL+L", TerminalTextMuted) { onSendRaw(byteArrayOf(0x0C)) }
                CompactButton("↑", TerminalAccent) { onSendKey("\u001B[A") }
                CompactButton("↓", TerminalAccent) { onSendKey("\u001B[B") }
                CompactButton("←", TerminalTextMuted) { onSendKey("\u001B[D") }
                CompactButton("→", TerminalTextMuted) { onSendKey("\u001B[C") }
                CompactButton("ENTER", TerminalGreen) { onSendKey("\r") }
            }

            Spacer(modifier = Modifier.height(4.dp))

            // Row 2: Claude Code Quick Prompts & Shell Shortcuts
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .horizontalScroll(rememberScrollState()),
                horizontalArrangement = Arrangement.spacedBy(4.dp)
            ) {
                CompactButton("✓ YES", TerminalGreen) { onSendKey("y\r") }
                CompactButton("✗ NO", TerminalRed) { onSendKey("n\r") }
                CompactButton("/compact", TerminalAccent) { onSendKey("/compact\r") }
                CompactButton("/cost", TerminalAmber) { onSendKey("/cost\r") }
                CompactButton("git status", TerminalPurpleBright) { onSendKey("git status\r") }
                CompactButton("/clear", TerminalTextMuted) { onSendKey("/clear\r") }
                CompactButton("Esc x2", TerminalRedBright) { onSendRaw(byteArrayOf(0x1B, 0x1B)) }
            }
        }
    }
}

@Composable
private fun CompactButton(
    text: String,
    contentColor: Color = Color.White,
    onClick: () -> Unit
) {
    FilledTonalButton(
        onClick = onClick,
        modifier = Modifier.height(28.dp),
        contentPadding = PaddingValues(horizontal = 8.dp, vertical = 0.dp),
        shape = RoundedCornerShape(6.dp),
        colors = ButtonDefaults.filledTonalButtonColors(
            containerColor = TerminalSurfaceElevated,
            contentColor = contentColor
        )
    ) {
        Text(
            text = text,
            fontFamily = FontFamily.Monospace,
            fontSize = 11.sp,
            fontWeight = FontWeight.Bold
        )
    }
}
