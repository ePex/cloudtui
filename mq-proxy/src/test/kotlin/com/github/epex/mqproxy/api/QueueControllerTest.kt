package com.github.epex.mqproxy.api

import com.github.epex.mqproxy.api.model.DeletedMessageDto
import com.github.epex.mqproxy.api.model.MessageSummary
import com.github.epex.mqproxy.api.model.MovedMessageDto
import com.github.epex.mqproxy.api.model.QueueMessageFilter
import com.github.epex.mqproxy.api.model.QueueSummary
import com.github.epex.mqproxy.config.ProxyAuthProperties
import com.github.epex.mqproxy.service.BrokerService
import com.ninjasquad.springmockk.MockkBean
import io.mockk.every
import jakarta.jms.JMSException
import org.junit.jupiter.api.Test
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.boot.context.properties.EnableConfigurationProperties
import org.springframework.boot.test.autoconfigure.web.servlet.WebMvcTest
import org.springframework.http.MediaType
import org.springframework.security.test.context.support.WithMockUser
import org.springframework.security.test.web.servlet.request.SecurityMockMvcRequestPostProcessors.csrf
import org.springframework.test.web.servlet.MockMvc
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
    // GET /api/management/command/list-queues
    // -------------------------------------------------------------------------

    @Test
    @WithMockUser
    fun `listQueues returns 200 with queue summaries`() {
        every { brokerService.listQueues() } returns listOf(
            QueueSummary("orders", messageCount = 5, consumerCount = 1, enqueuedCount = 10, dequeuedCount = 5, producerCount = 2),
        )

        mockMvc.get("/api/management/command/list-queues")
            .andExpect {
                status { isOk() }
                jsonPath("$.data[0].name") { value("orders") }
                jsonPath("$.data[0].messageCount") { value(5) }
                jsonPath("$.data[0].producerCount") { value(2) }
                jsonPath("$.errors") { isEmpty() }
            }
    }

    @Test
    fun `listQueues returns 401 when unauthenticated`() {
        mockMvc.get("/api/management/command/list-queues")
            .andExpect { status { isUnauthorized() } }
    }

    @Test
    @WithMockUser
    fun `listQueues returns 502 on JMSException`() {
        every { brokerService.listQueues() } throws JMSException("broker down")

        mockMvc.get("/api/management/command/list-queues")
            .andExpect {
                status { isBadGateway() }
                jsonPath("$.error") { value("broker down") }
            }
    }

    // -------------------------------------------------------------------------
    // GET /api/management/command/list-messages
    // -------------------------------------------------------------------------

    @Test
    @WithMockUser
    fun `listMessages returns 200 with message list`() {
        every { brokerService.browseMessages("orders", QueueMessageFilter(maxCount = 50)) } returns listOf(
            MessageSummary("orders", "ID:m1", "text", "hello", "2024-01-01T00:00:00Z", emptyMap()),
        )

        mockMvc.get("/api/management/command/list-messages?sourceQueue=orders&filter.maxCount=50")
            .andExpect {
                status { isOk() }
                jsonPath("$.data[0].messageId") { value("ID:m1") }
                jsonPath("$.data[0].jmsType") { value("text") }
                jsonPath("$.data[0].body") { value("hello") }
            }
    }

    @Test
    @WithMockUser
    fun `listMessages passes jmsType and messageId filters through`() {
        every {
            brokerService.browseMessages(
                "orders",
                QueueMessageFilter(jmsType = "order-created", messageId = "ID:m1", maxCount = 50),
            )
        } returns emptyList()

        mockMvc.get("/api/management/command/list-messages?sourceQueue=orders&filter.jmsType=order-created&filter.messageId=ID:m1&filter.maxCount=50")
            .andExpect { status { isOk() } }
    }

    @Test
    @WithMockUser
    fun `listMessages returns 400 when sourceQueue is missing`() {
        mockMvc.get("/api/management/command/list-messages")
            .andExpect { status { isBadRequest() } }
    }

    @Test
    @WithMockUser
    fun `listMessages returns 400 when maxCount is missing`() {
        mockMvc.get("/api/management/command/list-messages?sourceQueue=orders")
            .andExpect { status { isBadRequest() } }
    }

    @Test
    @WithMockUser
    fun `listMessages returns 400 when maxCount is not positive`() {
        mockMvc.get("/api/management/command/list-messages?sourceQueue=orders&filter.maxCount=0")
            .andExpect { status { isBadRequest() } }
    }

    @Test
    @WithMockUser
    fun `listMessages returns 502 on JMSException`() {
        every {
            brokerService.browseMessages("orders", QueueMessageFilter(maxCount = 50))
        } throws JMSException("connection lost")

        mockMvc.get("/api/management/command/list-messages?sourceQueue=orders&filter.maxCount=50")
            .andExpect { status { isBadGateway() } }
    }

    // -------------------------------------------------------------------------
    // POST /api/management/command/send-message
    // -------------------------------------------------------------------------

    @Test
    @WithMockUser
    fun `sendMessage returns 200 with the generated message id`() {
        every {
            brokerService.sendMessage(match { it.targetQueue == "orders" && it.body == "hello world" })
        } returns "ID:sent-1"

        mockMvc.post("/api/management/command/send-message") {
            with(csrf())
            contentType = MediaType.APPLICATION_JSON
            content = """{"targetQueue":"orders","jmsType":"text","body":"hello world"}"""
        }.andExpect {
            status { isOk() }
            jsonPath("$.data.messageId") { value("ID:sent-1") }
        }
    }

    // -------------------------------------------------------------------------
    // POST /api/management/command/delete-messages
    // -------------------------------------------------------------------------

    @Test
    @WithMockUser
    fun `deleteMessages returns 200 with deleted message ids`() {
        every {
            brokerService.deleteMessages("orders", QueueMessageFilter(messageId = "ID:m1", maxCount = 1))
        } returns listOf(DeletedMessageDto("ID:m1"))

        mockMvc.post("/api/management/command/delete-messages") {
            with(csrf())
            contentType = MediaType.APPLICATION_JSON
            content = """[{"sourceQueue":"orders","filter":{"messageId":"ID:m1","maxCount":1}}]"""
        }.andExpect {
            status { isOk() }
            jsonPath("$.data[0].messageId") { value("ID:m1") }
        }
    }

    @Test
    @WithMockUser
    fun `deleteMessages with an empty filter purges the queue`() {
        every {
            brokerService.deleteMessages("orders", QueueMessageFilter())
        } returns listOf(DeletedMessageDto("ID:1"), DeletedMessageDto("ID:2"))

        mockMvc.post("/api/management/command/delete-messages") {
            with(csrf())
            contentType = MediaType.APPLICATION_JSON
            content = """[{"sourceQueue":"orders","filter":{}}]"""
        }.andExpect {
            status { isOk() }
            jsonPath("$.data.length()") { value(2) }
        }
    }

    // -------------------------------------------------------------------------
    // POST /api/management/command/move-messages
    // -------------------------------------------------------------------------

    @Test
    @WithMockUser
    fun `moveMessages returns 200 with moved message ids`() {
        every {
            brokerService.moveMessages("orders", "archive", QueueMessageFilter())
        } returns listOf(MovedMessageDto("ID:1"), MovedMessageDto("ID:2"))

        mockMvc.post("/api/management/command/move-messages") {
            with(csrf())
            contentType = MediaType.APPLICATION_JSON
            content = """[{"sourceQueue":"orders","targetQueue":"archive","filter":{}}]"""
        }.andExpect {
            status { isOk() }
            jsonPath("$.data.length()") { value(2) }
        }
    }

    @Test
    @WithMockUser
    fun `moveMessages returns 502 on JMSException`() {
        every {
            brokerService.moveMessages("orders", "dlq", QueueMessageFilter(messageId = "ID:m1", maxCount = 1))
        } throws JMSException("broker error")

        mockMvc.post("/api/management/command/move-messages") {
            with(csrf())
            contentType = MediaType.APPLICATION_JSON
            content = """[{"sourceQueue":"orders","targetQueue":"dlq","filter":{"messageId":"ID:m1","maxCount":1}}]"""
        }.andExpect { status { isBadGateway() } }
    }
}
