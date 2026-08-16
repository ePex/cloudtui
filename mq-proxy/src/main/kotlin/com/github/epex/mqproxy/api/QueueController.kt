package com.github.epex.mqproxy.api

import com.github.epex.mqproxy.api.model.DeleteMessagesRequest
import com.github.epex.mqproxy.api.model.ItemResponse
import com.github.epex.mqproxy.api.model.ListResponse
import com.github.epex.mqproxy.api.model.MoveMessagesRequest
import com.github.epex.mqproxy.api.model.QueueMessageFilter
import com.github.epex.mqproxy.api.model.SendMessageRequest
import com.github.epex.mqproxy.api.model.SendMessageResponseDto
import com.github.epex.mqproxy.service.BrokerService
import org.springframework.web.bind.annotation.GetMapping
import org.springframework.web.bind.annotation.ModelAttribute
import org.springframework.web.bind.annotation.PostMapping
import org.springframework.web.bind.annotation.RequestBody
import org.springframework.web.bind.annotation.RequestMapping
import org.springframework.web.bind.annotation.RestController

/**
 * Query params for GET list-messages, bound via @ModelAttribute:
 * `sourceQueue`/`returnBody` stay top-level, `filter`'s fields nest as
 * `filter.jmsType`, `filter.maxCount`, etc. — matching the reference
 * API's binding convention and mq-proxy's own delete/move-messages JSON
 * bodies (see spec/49-cr-mq-proxy-nested-filter-query). `sourceQueue`
 * being required is what makes Spring constructor-bind the whole tree,
 * including the nested `filter`; a bare `QueueMessageFilter` parameter
 * (all fields optional) does not get the same nested binding on its own.
 */
data class ListMessagesQuery(
    val sourceQueue: String,
    val returnBody: Boolean? = null,
    val filter: QueueMessageFilter = QueueMessageFilter(),
)

@RestController
@RequestMapping("/api/management/command")
class QueueController(private val brokerService: BrokerService) {

    @GetMapping("/list-queues")
    fun listQueues() = ListResponse(data = brokerService.listQueues())

    @GetMapping("/list-messages")
    fun listMessages(@ModelAttribute query: ListMessagesQuery) = ListResponse(
        data = brokerService.browseMessages(
            query.sourceQueue,
            query.filter,
            returnBody = query.returnBody ?: true,
        ),
    )

    @PostMapping("/send-message")
    fun sendMessage(@RequestBody request: SendMessageRequest) =
        ItemResponse(data = SendMessageResponseDto(messageId = brokerService.sendMessage(request)))

    @PostMapping("/delete-messages")
    fun deleteMessages(@RequestBody requests: List<DeleteMessagesRequest>) = ListResponse(
        data = requests.flatMap { brokerService.deleteMessages(it.sourceQueue, it.filter) },
    )

    @PostMapping("/move-messages")
    fun moveMessages(@RequestBody requests: List<MoveMessagesRequest>) = ListResponse(
        data = requests.flatMap { brokerService.moveMessages(it.sourceQueue, it.targetQueue, it.filter) },
    )
}
