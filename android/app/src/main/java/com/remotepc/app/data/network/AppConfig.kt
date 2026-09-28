package com.remotepc.app.data.network

import android.content.Context
import android.content.SharedPreferences

class AppConfig(context: Context) {
    private val prefs: SharedPreferences =
        context.getSharedPreferences("remotepc_prefs", Context.MODE_PRIVATE)

    var host: String
        get() = prefs.getString("host", "192.168.1.100") ?: "192.168.1.100"
        set(value) = prefs.edit().putString("host", value.trim()).apply()

    var port: String
        get() = prefs.getString("port", "8080") ?: "8080"
        set(value) = prefs.edit().putString("port", value.trim()).apply()

    var token: String
        get() = prefs.getString("token", "") ?: ""
        set(value) = prefs.edit().putString("token", value.trim()).apply()

    val baseUrl: String
        get() = "http://$host:$port"

    val wsUrl: String
        get() = "ws://$host:$port"
}
