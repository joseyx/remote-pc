package com.remotepc.app.ui.components

import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
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
import com.remotepc.app.ui.theme.TerminalAccent
import com.remotepc.app.ui.theme.TerminalAmber
import com.remotepc.app.ui.theme.TerminalGreen
import com.remotepc.app.ui.theme.TerminalRed
import com.remotepc.app.ui.theme.TerminalSurface

@Composable
fun ClaudeShortcutBar(
    onSendKey: (String) -> Unit,
    onSendRaw: (ByteArray) -> Unit,
    modifier: Modifier = Modifier
) {
    Surface(
        modifier = modifier.fillMaxWidth(),
        color = Color(0xFF161B22), // GitHub dark secondary
        tonalElevation = 2.dp
    ) {
        Column(modifier = Modifier.padding(horizontal = 4.dp, vertical = 2.dp)) {
            // Row 1: Special Navigation Keys (Esc, Tab, Ctrl+C, Arrows, Enter)
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .horizontalScroll(rememberScrollState()),
                horizontalArrangement = Arrangement.spacedBy(3.dp)
            ) {
                CompactButton("ESC", Color(0xFFE2E8F0)) { onSendRaw(byteArrayOf(0x1B)) }
                CompactButton("TAB", Color(0xFFE2E8F0)) { onSendKey("\t") }
                CompactButton("CTRL+C", TerminalRed) { onSendRaw(byteArrayOf(0x03)) }
                CompactButton("CTRL+D", Color(0xFF94A3B8)) { onSendRaw(byteArrayOf(0x04)) }
                CompactButton("↑", TerminalAccent) { onSendKey("\u001B[A") }
                CompactButton("↓", TerminalAccent) { onSendKey("\u001B[B") }
                CompactButton("←", Color(0xFFCBD5E1)) { onSendKey("\u001B[D") }
                CompactButton("→", Color(0xFFCBD5E1)) { onSendKey("\u001B[C") }
                CompactButton("ENTER", TerminalGreen) { onSendKey("\r") }
            }

            // Row 2: Claude Code Quick Prompts
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(top = 2.dp)
                    .horizontalScroll(rememberScrollState()),
                horizontalArrangement = Arrangement.spacedBy(3.dp)
            ) {
                CompactButton("✓ YES", TerminalGreen) { onSendKey("y\r") }
                CompactButton("✗ NO", TerminalRed) { onSendKey("n\r") }
                CompactButton("/cost", TerminalAmber) { onSendKey("/cost\r") }
                CompactButton("/compact", TerminalAccent) { onSendKey("/compact\r") }
                CompactButton("/help", Color(0xFF94A3B8)) { onSendKey("/help\r") }
                CompactButton("/clear", Color(0xFF94A3B8)) { onSendKey("/clear\r") }
                CompactButton("Esc x2", Color(0xFFF43F5E)) { onSendRaw(byteArrayOf(0x1B, 0x1B)) }
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
        shape = RoundedCornerShape(4.dp),
        colors = ButtonDefaults.filledTonalButtonColors(
            containerColor = Color(0xFF21262D),
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
