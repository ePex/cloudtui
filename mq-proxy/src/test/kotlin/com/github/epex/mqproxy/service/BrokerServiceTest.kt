package com.github.epex.mqproxy.service

import com.github.epex.mqproxy.api.model.QueueMessageFilter
import com.github.epex.mqproxy.api.model.SendMessageRequest
import io.mockk.every
import io.mockk.just
import io.mockk.mockk
import io.mockk.Runs
import io.mockk.verify
import jakarta.jms.Connection
import jakarta.jms.Message
import jakarta.jms.MessageConsumer
import jakarta.jms.MessageProducer
import jakarta.jms.Queue
import jakarta.jms.QueueBrowser
import jakarta.jms.Session
import jakarta.jms.TemporaryQueue
import jakarta.jms.TextMessage
import org.apache.activemq.ActiveMQConnection
import org.apache.activemq.ActiveMQConnectionFactory
import org.apache.activemq.advisory.DestinationSource
import org.apache.activemq.command.ActiveMQQueue
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import java.util.Collections

class BrokerServiceTest {

    private val factory = mockk<ActiveMQConnectionFactory>()
    private val service = BrokerService(factory)

    // -------------------------------------------------------------------------
    // Helpers
    // -------------------------------------------------------------------------

    private fun mockConnection(): Connection {
        val conn = mockk<Connection>()
        every { factory.createConnection() } returns conn
        every { conn.start() } just Runs
        every { conn.close() } just Runs
        return conn
    }

    private fun mockAmqConnection(): ActiveMQConnection {
        val conn = mockk<ActiveMQConnection>()
        every { factory.createConnection() } returns conn
        every { conn.start() } just Runs
        every { conn.close() } just Runs
        return conn
    }

    private fun stubSession(conn: Connection, transacted: Boolean = false): Session {
        val session = mockk<Session>()
        val mode = if (transacted) Session.SESSION_TRANSACTED else Session.AUTO_ACKNOWLEDGE
        every { conn.createSession(transacted, mode) } returns session
        return session
    }

    private fun stubTextMessage(
        session: Session,
        id: String,
        timestamp: Long = 0L,
        body: String = "body",
        jmsType: String = "text",
    ): TextMessage {
        val msg = mockk<TextMessage>()
        every { msg.jmsMessageID } returns id
        every { msg.jmsTimestamp } returns timestamp
        every { msg.text } returns body
        every { msg.jmsType } returns jmsType
        every { msg.propertyNames } returns Collections.emptyEnumeration<Any>()
        return msg
    }

    private fun stubBrowser(session: Session, queueName: String, selector: String?, messages: List<Message>): QueueBrowser {
        val queue = mockk<Queue>()
        val browser = mockk<QueueBrowser>()
        every { session.createQueue(queueName) } returns queue
        every { session.createBrowser(queue, selector) } returns browser
        every { browser.enumeration } returns Collections.enumeration(messages)
        every { browser.close() } just Runs
        return browser
    }

    // -------------------------------------------------------------------------
    // listQueues
    // -------------------------------------------------------------------------

    @Test
    fun `listQueues returns queue summary with counts minus one when stats unavailable`() {
        val conn = mockAmqConnection()
        val session = stubSession(conn)

        val destSource = mockk<DestinationSource>()
        val activeMqQueue = mockk<ActiveMQQueue>()
        every { conn.destinationSource } returns destSource
        every { destSource.queues } returns setOf(activeMqQueue)
        every { activeMqQueue.physicalName } returns "test.queue"

        // Stats plugin not available — temp queue consumer times out
        val tempQueue = mockk<TemporaryQueue>()
        val statsConsumer = mockk<MessageConsumer>()
        val statsProducer = mockk<MessageProducer>()
        val statsQueue = mockk<Queue>()
        val statsRequest = mockk<Message>(relaxed = true)
        every { session.createTemporaryQueue() } returns tempQueue
        every { session.createConsumer(tempQueue) } returns statsConsumer
        every { session.createQueue("ActiveMQ.Statistics.Destination.test.queue") } returns statsQueue
        every { session.createProducer(statsQueue) } returns statsProducer
        every { session.createMessage() } returns statsRequest
        every { statsProducer.send(statsRequest) } just Runs
        every { statsConsumer.receive(3_000) } returns null
        every { statsProducer.close() } just Runs
        every { statsConsumer.close() } just Runs
        every { tempQueue.delete() } just Runs

        val result = service.listQueues()

        assertThat(result).hasSize(1)
        assertThat(result[0].name).isEqualTo("test.queue")
        assertThat(result[0].messageCount).isEqualTo(-1L)
        assertThat(result[0].consumerCount).isEqualTo(-1L)
        assertThat(result[0].producerCount).isEqualTo(-1L)
    }

    @Test
    fun `listQueues filters out ActiveMQ internal queues`() {
        val conn = mockAmqConnection()
        val session = stubSession(conn)

        val destSource = mockk<DestinationSource>()
        val internalQueue = mockk<ActiveMQQueue>()
        val userQueue = mockk<ActiveMQQueue>()
        every { conn.destinationSource } returns destSource
        every { destSource.queues } returns setOf(internalQueue, userQueue)
        every { internalQueue.physicalName } returns "ActiveMQ.Advisory.Queue"
        every { userQueue.physicalName } returns "my.queue"

        val tempQueue = mockk<TemporaryQueue>()
        val statsConsumer = mockk<MessageConsumer>()
        val statsProducer = mockk<MessageProducer>()
        val statsQueue = mockk<Queue>()
        val statsRequest = mockk<Message>(relaxed = true)
        every { session.createTemporaryQueue() } returns tempQueue
        every { session.createConsumer(tempQueue) } returns statsConsumer
        every { session.createQueue(any()) } returns statsQueue
        every { session.createProducer(statsQueue) } returns statsProducer
        every { session.createMessage() } returns statsRequest
        every { statsProducer.send(statsRequest) } just Runs
        every { statsConsumer.receive(3_000) } returns null
        every { statsProducer.close() } just Runs
        every { statsConsumer.close() } just Runs
        every { tempQueue.delete() } just Runs

        val result = service.listQueues()

        assertThat(result).hasSize(1)
        assertThat(result[0].name).isEqualTo("my.queue")
    }

    // -------------------------------------------------------------------------
    // browseMessages
    // -------------------------------------------------------------------------

    @Test
    fun `browseMessages maps TextMessage fields to MessageSummary`() {
        val conn = mockConnection()
        val session = stubSession(conn)
        val msg = stubTextMessage(session, id = "ID:test-1", timestamp = 1_700_000_000_000L, body = "hello", jmsType = "order-created")
        stubBrowser(session, "orders", null, listOf(msg))

        val result = service.browseMessages("orders", QueueMessageFilter())

        assertThat(result).hasSize(1)
        assertThat(result[0].sourceQueue).isEqualTo("orders")
        assertThat(result[0].messageId).isEqualTo("ID:test-1")
        assertThat(result[0].jmsType).isEqualTo("order-created")
        assertThat(result[0].body).isEqualTo("hello")
        assertThat(result[0].timestamp).startsWith("2023-")
    }

    @Test
    fun `browseMessages returns empty list for empty queue`() {
        val conn = mockConnection()
        val session = stubSession(conn)
        stubBrowser(session, "empty.queue", null, emptyList())

        val result = service.browseMessages("empty.queue", QueueMessageFilter())

        assertThat(result).isEmpty()
    }

    @Test
    fun `browseMessages passes a selector built from the filter`() {
        val conn = mockConnection()
        val session = stubSession(conn)
        stubBrowser(session, "orders", "JMSType = 'order-created'", emptyList())

        service.browseMessages("orders", QueueMessageFilter(jmsType = "order-created"))

        verify { session.createBrowser(any(), "JMSType = 'order-created'") }
    }

    @Test
    fun `browseMessages stops after maxCount matches`() {
        val conn = mockConnection()
        val session = stubSession(conn)
        val msg1 = stubTextMessage(session, id = "ID:1")
        val msg2 = stubTextMessage(session, id = "ID:2")
        stubBrowser(session, "orders", null, listOf(msg1, msg2))

        val result = service.browseMessages("orders", QueueMessageFilter(maxCount = 1))

        assertThat(result.map { it.messageId }).containsExactly("ID:1")
    }

    @Test
    fun `browseMessages omits body when returnBody is false`() {
        val conn = mockConnection()
        val session = stubSession(conn)
        val msg = stubTextMessage(session, id = "ID:1", body = "hello")
        stubBrowser(session, "orders", null, listOf(msg))

        val result = service.browseMessages("orders", QueueMessageFilter(), returnBody = false)

        assertThat(result[0].body).isNull()
    }

    @Test
    fun `browseMessages includes body by default`() {
        val conn = mockConnection()
        val session = stubSession(conn)
        val msg = stubTextMessage(session, id = "ID:1", body = "hello")
        stubBrowser(session, "orders", null, listOf(msg))

        val result = service.browseMessages("orders", QueueMessageFilter())

        assertThat(result[0].body).isEqualTo("hello")
    }

    // -------------------------------------------------------------------------
    // deleteMessages
    // -------------------------------------------------------------------------

    @Test
    fun `deleteMessages with an empty filter consumes every message`() {
        val conn = mockConnection()
        val session = stubSession(conn)
        val queue = mockk<Queue>()
        val consumer = mockk<MessageConsumer>()
        val msg1 = mockk<Message>().also { every { it.jmsMessageID } returns "ID:1" }
        val msg2 = mockk<Message>().also { every { it.jmsMessageID } returns "ID:2" }

        every { session.createQueue("trash") } returns queue
        every { session.createConsumer(queue, null) } returns consumer
        every { consumer.receiveNoWait() } returnsMany listOf(msg1, msg2, null)
        every { consumer.close() } just Runs

        val result = service.deleteMessages("trash", QueueMessageFilter())

        assertThat(result.map { it.messageId }).containsExactly("ID:1", "ID:2")
    }

    @Test
    fun `deleteMessages returns empty list for empty queue`() {
        val conn = mockConnection()
        val session = stubSession(conn)
        val queue = mockk<Queue>()
        val consumer = mockk<MessageConsumer>()

        every { session.createQueue("trash") } returns queue
        every { session.createConsumer(queue, null) } returns consumer
        every { consumer.receiveNoWait() } returns null
        every { consumer.close() } just Runs

        assertThat(service.deleteMessages("trash", QueueMessageFilter())).isEmpty()
    }

    @Test
    fun `deleteMessages stops after maxCount matches`() {
        val conn = mockConnection()
        val session = stubSession(conn)
        val queue = mockk<Queue>()
        val consumer = mockk<MessageConsumer>()
        val msg1 = mockk<Message>().also { every { it.jmsMessageID } returns "ID:1" }
        val msg2 = mockk<Message>().also { every { it.jmsMessageID } returns "ID:2" }

        every { session.createQueue("trash") } returns queue
        every { session.createConsumer(queue, null) } returns consumer
        every { consumer.receiveNoWait() } returnsMany listOf(msg1, msg2)
        every { consumer.close() } just Runs

        val result = service.deleteMessages("trash", QueueMessageFilter(maxCount = 1))

        assertThat(result.map { it.messageId }).containsExactly("ID:1")
        verify(exactly = 1) { consumer.receiveNoWait() }
    }

    @Test
    fun `deleteMessages builds a selector combining jmsType, messageId, and date range`() {
        val conn = mockConnection()
        val session = stubSession(conn)
        val queue = mockk<Queue>()
        val consumer = mockk<MessageConsumer>()

        every { session.createQueue("orders") } returns queue
        every { session.createConsumer(queue, any()) } returns consumer
        every { consumer.receiveNoWait() } returns null
        every { consumer.close() } just Runs

        service.deleteMessages(
            "orders",
            QueueMessageFilter(
                jmsType = "order-created",
                messageId = "ID:x",
                fromDate = "2023-11-14T22:13:20Z",
                toDate = "2023-11-15T22:13:20Z",
            ),
        )

        verify {
            session.createConsumer(
                queue,
                "JMSType = 'order-created' AND JMSMessageID = 'ID:x' AND JMSTimestamp >= 1700000000000 AND JMSTimestamp <= 1700086400000",
            )
        }
    }

    // -------------------------------------------------------------------------
    // moveMessages
    // -------------------------------------------------------------------------

    @Test
    fun `moveMessages with an empty filter moves every message and commits`() {
        val conn = mockConnection()
        val session = stubSession(conn, transacted = true)
        val srcQueue = mockk<Queue>()
        val dstQueue = mockk<Queue>()
        val consumer = mockk<MessageConsumer>()
        val producer = mockk<MessageProducer>()
        val msg1 = mockk<Message>().also { every { it.jmsMessageID } returns "ID:1" }
        val msg2 = mockk<Message>().also { every { it.jmsMessageID } returns "ID:2" }

        every { session.createQueue("src") } returns srcQueue
        every { session.createQueue("dst") } returns dstQueue
        every { session.createConsumer(srcQueue, null) } returns consumer
        every { session.createProducer(dstQueue) } returns producer
        every { consumer.receiveNoWait() } returnsMany listOf(msg1, msg2, null)
        every { producer.send(any()) } just Runs
        every { session.commit() } just Runs
        every { producer.close() } just Runs
        every { consumer.close() } just Runs

        val result = service.moveMessages("src", "dst", QueueMessageFilter())

        assertThat(result.map { it.messageId }).containsExactly("ID:1", "ID:2")
        verify(exactly = 2) { producer.send(any()) }
        verify { session.commit() }
    }

    @Test
    fun `moveMessages returns empty list and still commits for empty queue`() {
        val conn = mockConnection()
        val session = stubSession(conn, transacted = true)
        val srcQueue = mockk<Queue>()
        val dstQueue = mockk<Queue>()
        val consumer = mockk<MessageConsumer>()
        val producer = mockk<MessageProducer>()

        every { session.createQueue("src") } returns srcQueue
        every { session.createQueue("dst") } returns dstQueue
        every { session.createConsumer(srcQueue, null) } returns consumer
        every { session.createProducer(dstQueue) } returns producer
        every { consumer.receiveNoWait() } returns null
        every { session.commit() } just Runs
        every { producer.close() } just Runs
        every { consumer.close() } just Runs

        assertThat(service.moveMessages("src", "dst", QueueMessageFilter())).isEmpty()
        verify { session.commit() }
    }

    @Test
    fun `moveMessages stops after maxCount matches`() {
        val conn = mockConnection()
        val session = stubSession(conn, transacted = true)
        val srcQueue = mockk<Queue>()
        val dstQueue = mockk<Queue>()
        val consumer = mockk<MessageConsumer>()
        val producer = mockk<MessageProducer>()
        val msg1 = mockk<Message>().also { every { it.jmsMessageID } returns "ID:1" }
        val msg2 = mockk<Message>().also { every { it.jmsMessageID } returns "ID:2" }

        every { session.createQueue("src") } returns srcQueue
        every { session.createQueue("dst") } returns dstQueue
        every { session.createConsumer(srcQueue, null) } returns consumer
        every { session.createProducer(dstQueue) } returns producer
        every { consumer.receiveNoWait() } returnsMany listOf(msg1, msg2)
        every { producer.send(any()) } just Runs
        every { session.commit() } just Runs
        every { producer.close() } just Runs
        every { consumer.close() } just Runs

        val result = service.moveMessages("src", "dst", QueueMessageFilter(maxCount = 1))

        assertThat(result.map { it.messageId }).containsExactly("ID:1")
        verify(exactly = 1) { producer.send(any()) }
    }

    // -------------------------------------------------------------------------
    // sendMessage
    // -------------------------------------------------------------------------

    @Test
    fun `sendMessage sets jmsType, correlationId, groupId, and custom headers`() {
        val conn = mockConnection()
        val session = stubSession(conn)
        val queue = mockk<Queue>()
        val producer = mockk<MessageProducer>()
        val textMsg = mockk<TextMessage>()

        every { session.createQueue("outbox") } returns queue
        every { session.createProducer(queue) } returns producer
        every { session.createTextMessage("hello world") } returns textMsg
        every { textMsg.jmsType = "order-created" } just Runs
        every { textMsg.jmsCorrelationID = "corr-1" } just Runs
        every { textMsg.setStringProperty("JMSXGroupID", "group-1") } just Runs
        every { textMsg.setStringProperty("customHeader", "customValue") } just Runs
        every { producer.send(textMsg) } just Runs
        every { producer.close() } just Runs
        every { textMsg.jmsMessageID } returns "ID:sent-1"

        val messageId = service.sendMessage(
            SendMessageRequest(
                targetQueue = "outbox",
                jmsType = "order-created",
                headers = mapOf("customHeader" to "customValue"),
                groupId = "group-1",
                body = "hello world",
                correlationId = "corr-1",
            ),
        )

        assertThat(messageId).isEqualTo("ID:sent-1")
        verify { producer.send(textMsg) }
    }

    @Test
    fun `sendMessage without optional fields sets only jmsType`() {
        val conn = mockConnection()
        val session = stubSession(conn)
        val queue = mockk<Queue>()
        val producer = mockk<MessageProducer>()
        val textMsg = mockk<TextMessage>()

        every { session.createQueue("outbox") } returns queue
        every { session.createProducer(queue) } returns producer
        every { session.createTextMessage("hello world") } returns textMsg
        every { textMsg.jmsType = "text" } just Runs
        every { producer.send(textMsg) } just Runs
        every { producer.close() } just Runs
        every { textMsg.jmsMessageID } returns "ID:sent-2"

        service.sendMessage(SendMessageRequest(targetQueue = "outbox", jmsType = "text", body = "hello world"))

        verify(exactly = 0) { textMsg.jmsCorrelationID = any() }
        verify(exactly = 0) { textMsg.setStringProperty(any(), any()) }
    }
}
