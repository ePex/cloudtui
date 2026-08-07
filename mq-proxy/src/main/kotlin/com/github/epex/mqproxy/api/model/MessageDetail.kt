package com.github.epex.mqproxy.api.model

data class MessageDetail(
    val id: String,
    val timestamp: String,
    val body: String?,
    val deliveryMode: Int,
    val priority: Int,
    val correlationId: String?,
    val replyTo: String?,
    val destination: String,
    val redelivered: Boolean,
    val properties: Map<String, String>,
)
