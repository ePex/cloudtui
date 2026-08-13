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
import org.springframework.web.bind.annotation.PostMapping
import org.springframework.web.bind.annotation.RequestBody
import org.springframework.web.bind.annotation.RequestMapping
import org.springframework.web.bind.annotation.RequestParam
import org.springframework.web.bind.annotation.RestController

@RestController
@RequestMapping("/api/management/command")
class QueueController(private val brokerService: BrokerService) {

    @GetMapping("/list-queues")
    fun listQueues() = ListResponse(data = brokerService.listQueues())

    @GetMapping("/list-messages")
    fun listMessages(
        @RequestParam sourceQueue: String,
        @RequestParam(required = false) jmsType: String?,
        @RequestParam(required = false) messageId: String?,
        @RequestParam(required = false) fromDate: String?,
        @RequestParam(required = false) toDate: String?,
        @RequestParam(required = false) maxCount: Int?,
        @RequestParam(required = false) returnBody: Boolean?,
    ) = ListResponse(
        data = brokerService.browseMessages(
            sourceQueue,
            QueueMessageFilter(
                jmsType = jmsType,
                messageId = messageId,
                fromDate = fromDate,
                toDate = toDate,
                maxCount = maxCount,
            ),
            returnBody = returnBody ?: true,
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
