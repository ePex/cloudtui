package com.github.epex.mqproxy.api

import com.fasterxml.jackson.databind.JsonNode
import com.github.epex.mqproxy.api.model.MessageDetail
import com.github.epex.mqproxy.api.model.MessageSummary
import com.github.epex.mqproxy.api.model.QueueSummary
import com.github.epex.mqproxy.service.BrokerService
import org.springframework.http.HttpStatus
import org.springframework.web.bind.annotation.DeleteMapping
import org.springframework.web.bind.annotation.GetMapping
import org.springframework.web.bind.annotation.PathVariable
import org.springframework.web.bind.annotation.PostMapping
import org.springframework.web.bind.annotation.RequestBody
import org.springframework.web.bind.annotation.RequestMapping
import org.springframework.web.bind.annotation.RequestParam
import org.springframework.web.bind.annotation.ResponseStatus
import org.springframework.web.bind.annotation.RestController

@RestController
@RequestMapping("/api")
class QueueController(private val brokerService: BrokerService) {

    @GetMapping("/queues")
    fun listQueues(): List<QueueSummary> =
        brokerService.listQueues()

    @GetMapping("/queues/{name}/messages")
    fun browseMessages(@PathVariable name: String): List<MessageSummary> =
        brokerService.browseMessages(name)

    @GetMapping("/queues/{name}/messages/{id:.+}")
    fun getMessage(
        @PathVariable name: String,
        @PathVariable id: String,
    ): MessageDetail = brokerService.getMessage(name, id)

    @DeleteMapping("/queues/{name}/messages/{id:.+}")
    @ResponseStatus(HttpStatus.NO_CONTENT)
    fun deleteMessage(
        @PathVariable name: String,
        @PathVariable id: String,
    ) = brokerService.deleteMessage(name, id)

    @PostMapping("/queues/{name}/messages/{id:.+}/move")
    @ResponseStatus(HttpStatus.NO_CONTENT)
    fun moveMessage(
        @PathVariable name: String,
        @PathVariable id: String,
        @RequestParam to: String,
    ) = brokerService.moveMessage(name, id, to)

    @PostMapping("/queues/{name}/move")
    fun moveAll(
        @PathVariable name: String,
        @RequestParam to: String,
    ): Map<String, Int> = mapOf("moved" to brokerService.moveAll(name, to))

    @PostMapping("/queues/{name}/messages")
    @ResponseStatus(HttpStatus.CREATED)
    fun sendMessage(
        @PathVariable name: String,
        @RequestBody body: JsonNode,
    ) = brokerService.sendMessage(name, body.toString())

    @DeleteMapping("/queues/{name}/messages")
    fun purgeQueue(@PathVariable name: String): Map<String, Int> =
        mapOf("purged" to brokerService.purgeQueue(name))
}
