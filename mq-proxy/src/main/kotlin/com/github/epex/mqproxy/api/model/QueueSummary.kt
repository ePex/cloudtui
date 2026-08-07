package com.github.epex.mqproxy.api.model

data class QueueSummary(
    val name: String,
    val pendingCount: Long,
    val consumerCount: Long,
    val enqueueCount: Long,
    val dequeueCount: Long,
)
