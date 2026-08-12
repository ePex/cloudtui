package com.github.epex.mqproxy.api.model

data class DeleteMessagesRequest(
    val sourceQueue: String,
    val filter: QueueMessageFilter,
)
