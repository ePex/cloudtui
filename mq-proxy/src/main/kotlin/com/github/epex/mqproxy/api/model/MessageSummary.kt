package com.github.epex.mqproxy.api.model

data class MessageSummary(
    val id: String,
    val timestamp: String,
    val body: String?,
    val properties: Map<String, String>,
)
