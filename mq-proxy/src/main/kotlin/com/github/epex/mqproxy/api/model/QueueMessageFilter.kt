package com.github.epex.mqproxy.api.model

/**
 * Criteria for a bulk delete/move operation. Every field is optional;
 * an entirely empty filter matches every message on the queue (the
 * former purge/move-all behavior). [maxCount] caps how many matching
 * messages are consumed, oldest first.
 */
data class QueueMessageFilter(
    val jmsType: String? = null,
    val fromDate: String? = null,
    val toDate: String? = null,
    val messageId: String? = null,
    val maxCount: Int? = null,
)
