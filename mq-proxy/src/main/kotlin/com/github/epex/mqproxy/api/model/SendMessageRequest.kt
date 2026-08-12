package com.github.epex.mqproxy.api.model

data class SendMessageRequest(
    val targetQueue: String,
    val jmsType: String,
    val headers: Map<String, String>? = null,
    val groupId: String? = null,
    val body: String,
    val correlationId: String? = null,
)
