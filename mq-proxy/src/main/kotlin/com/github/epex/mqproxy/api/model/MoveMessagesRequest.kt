package com.github.epex.mqproxy.api.model

data class MoveMessagesRequest(
    val sourceQueue: String,
    val targetQueue: String,
    val filter: QueueMessageFilter,
)
