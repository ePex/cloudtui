package com.github.epex.mqproxy.api.model

/**
 * Criteria for a bulk delete/move operation, or for paging through
 * list-messages. Every field is optional; an entirely empty filter
 * matches every message on the queue (the former purge/move-all
 * behavior). [maxCount] caps how many matching messages are consumed,
 * oldest first.
 *
 * [afterMessageId] pages a list-messages call: when set, matching
 * starts just after the message with this ID rather than from the
 * beginning of the queue. Distinct from [messageId] (which selects
 * exactly one message) - reusing that field for a cursor would be a
 * confusing, breaking overload of its existing meaning.
 */
data class QueueMessageFilter(
    val jmsType: String? = null,
    val fromDate: String? = null,
    val toDate: String? = null,
    val messageId: String? = null,
    val maxCount: Int? = null,
    val afterMessageId: String? = null,
)
