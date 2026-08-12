package com.github.epex.mqproxy.api.model

data class MessageSummary(
    val sourceQueue: String,
    val messageId: String,
    val jmsType: String,
    val body: String?,
    val timestamp: String,
    val headers: Map<String, String>?,
)
