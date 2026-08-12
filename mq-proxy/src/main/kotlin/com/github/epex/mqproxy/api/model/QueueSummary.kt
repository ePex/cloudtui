package com.github.epex.mqproxy.api.model

data class QueueSummary(
    val name: String,
    val messageCount: Long,
    val consumerCount: Long,
    val enqueuedCount: Long,
    val dequeuedCount: Long,
    val producerCount: Long,
)
