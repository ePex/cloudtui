package com.github.epex.mqproxy.api

import com.github.epex.mqproxy.api.model.MessageDetail
import com.github.epex.mqproxy.api.model.MessageSummary
import com.github.epex.mqproxy.api.model.QueueSummary
import com.github.epex.mqproxy.config.ProxyAuthProperties
import com.github.epex.mqproxy.service.BrokerService
import com.github.epex.mqproxy.service.NotFoundException
import com.ninjasquad.springmockk.MockkBean
import io.mockk.every
import io.mockk.just
import io.mockk.Runs
import jakarta.jms.JMSException
import org.junit.jupiter.api.Test
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.boot.context.properties.EnableConfigurationProperties
import org.springframework.boot.test.autoconfigure.web.servlet.WebMvcTest
import org.springframework.http.MediaType
import org.springframework.security.test.context.support.WithMockUser
import org.springframework.security.test.web.servlet.request.SecurityMockMvcRequestPostProcessors.csrf
import org.springframework.test.web.servlet.MockMvc
import org.springframework.test.web.servlet.delete
import org.springframework.test.web.servlet.get
import org.springframework.test.web.servlet.post

@WebMvcTest(QueueController::class)
@EnableConfigurationProperties(ProxyAuthProperties::class)
class QueueControllerTest {

    @Autowired
    private lateinit var mockMvc: MockMvc

    @MockkBean
    private lateinit var brokerService: BrokerService

    // -------------------------------------------------------------------------
    // GET /api/queues
    // -------------------------------------------------------------------------

    @Test
    @WithMockUser
    fun `listQueues returns 200 with queue summaries`() {
        every { brokerService.listQueues() } returns listOf(
            QueueSummary("orders", 5, 1, 10, 5),
        )

        mockMvc.get("/api/queues")
            .andExpect {
                status { isOk() }
                jsonPath("$[0].name") { value("orders") }
                jsonPath("$[0].pendingCount") { value(5) }
            }
    }

    @Test
    fun `listQueues returns 401 when unauthenticated`() {
        mockMvc.get("/api/queues")
            .andExpect { status { isUnauthorized() } }
    }

    @Test
    @WithMockUser
    fun `listQueues returns 502 on JMSException`() {
        every { brokerService.listQueues() } throws JMSException("broker down")

        mockMvc.get("/api/queues")
            .andExpect {
                status { isBadGateway() }
                jsonPath("$.error") { value("broker down") }
            }
    }

    // -------------------------------------------------------------------------
    // GET /api/queues/{name}/messages
    // -------------------------------------------------------------------------

    @Test
    @WithMockUser
    fun `browseMessages returns 200 with message list`() {
        every { brokerService.browseMessages("orders") } returns listOf(
            MessageSummary("ID:m1", "2024-01-01T00:00:00Z", "hello", emptyMap()),
        )

        mockMvc.get("/api/queues/orders/messages")
            .andExpect {
                status { isOk() }
                jsonPath("$[0].id") { value("ID:m1") }
                jsonPath("$[0].body") { value("hello") }
            }
    }

    @Test
    @WithMockUser
    fun `browseMessages returns 502 on JMSException`() {
        every { brokerService.browseMessages("orders") } throws JMSException("connection lost")

        mockMvc.get("/api/queues/orders/messages")
            .andExpect { status { isBadGateway() } }
    }

    // -------------------------------------------------------------------------
    // GET /api/queues/{name}/messages/{id}
    // -------------------------------------------------------------------------

    @Test
    @WithMockUser
    fun `getMessage returns 200 with message detail`() {
        val detail = MessageDetail(
            id = "ID:abc-1",
            timestamp = "2024-01-01T00:00:00Z",
            body = "payload",
            deliveryMode = 2,
            priority = 4,
            correlationId = null,
            replyTo = null,
            destination = "queue://orders",
            redelivered = false,
            properties = emptyMap(),
        )
        every { brokerService.getMessage("orders", "ID:abc-1") } returns detail

        mockMvc.get("/api/queues/orders/messages/ID:abc-1")
            .andExpect {
                status { isOk() }
                jsonPath("$.id") { value("ID:abc-1") }
                jsonPath("$.body") { value("payload") }
            }
    }

    @Test
    @WithMockUser
    fun `getMessage returns 404 when not found`() {
        every { brokerService.getMessage("orders", "ID:missing") } throws
            NotFoundException("Message not found")

        mockMvc.get("/api/queues/orders/messages/ID:missing")
            .andExpect {
                status { isNotFound() }
                jsonPath("$.error") { value("Message not found") }
            }
    }

    // -------------------------------------------------------------------------
    // DELETE /api/queues/{name}/messages/{id}
    // -------------------------------------------------------------------------

    @Test
    @WithMockUser
    fun `deleteMessage returns 204 on success`() {
        every { brokerService.deleteMessage("orders", "ID:m1") } just Runs

        mockMvc.delete("/api/queues/orders/messages/ID:m1") { with(csrf()) }
            .andExpect { status { isNoContent() } }
    }

    @Test
    @WithMockUser
    fun `deleteMessage returns 404 when not found`() {
        every { brokerService.deleteMessage("orders", "ID:gone") } throws
            NotFoundException("Message not found")

        mockMvc.delete("/api/queues/orders/messages/ID:gone") { with(csrf()) }
            .andExpect { status { isNotFound() } }
    }

    // -------------------------------------------------------------------------
    // POST /api/queues/{name}/messages/{id}/move
    // -------------------------------------------------------------------------

    @Test
    @WithMockUser
    fun `moveMessage returns 204 on success`() {
        every { brokerService.moveMessage("orders", "ID:m1", "dlq") } just Runs

        mockMvc.post("/api/queues/orders/messages/ID:m1/move?to=dlq") { with(csrf()) }
            .andExpect { status { isNoContent() } }
    }

    @Test
    @WithMockUser
    fun `moveMessage returns 404 when not found`() {
        every { brokerService.moveMessage("orders", "ID:gone", "dlq") } throws
            NotFoundException("Message not found")

        mockMvc.post("/api/queues/orders/messages/ID:gone/move?to=dlq") { with(csrf()) }
            .andExpect { status { isNotFound() } }
    }

    // -------------------------------------------------------------------------
    // POST /api/queues/{name}/move
    // -------------------------------------------------------------------------

    @Test
    @WithMockUser
    fun `moveAll returns 200 with moved count`() {
        every { brokerService.moveAll("orders", "archive") } returns 7

        mockMvc.post("/api/queues/orders/move?to=archive") { with(csrf()) }
            .andExpect {
                status { isOk() }
                jsonPath("$.moved") { value(7) }
            }
    }

    // -------------------------------------------------------------------------
    // POST /api/queues/{name}/messages
    // -------------------------------------------------------------------------

    @Test
    @WithMockUser
    fun `sendMessage returns 201 on success`() {
        every { brokerService.sendMessage("orders", """{"text":"hello world"}""") } just Runs

        mockMvc.post("/api/queues/orders/messages") {
            with(csrf())
            contentType = MediaType.APPLICATION_JSON
            content = """{"text":"hello world"}"""
        }.andExpect { status { isCreated() } }
    }

    // -------------------------------------------------------------------------
    // DELETE /api/queues/{name}/messages
    // -------------------------------------------------------------------------

    @Test
    @WithMockUser
    fun `purgeQueue returns 200 with purged count`() {
        every { brokerService.purgeQueue("orders") } returns 12

        mockMvc.delete("/api/queues/orders/messages") { with(csrf()) }
            .andExpect {
                status { isOk() }
                jsonPath("$.purged") { value(12) }
            }
    }

    @Test
    @WithMockUser
    fun `purgeQueue returns 502 on JMSException`() {
        every { brokerService.purgeQueue("orders") } throws JMSException("broker error")

        mockMvc.delete("/api/queues/orders/messages") { with(csrf()) }
            .andExpect { status { isBadGateway() } }
    }
}
