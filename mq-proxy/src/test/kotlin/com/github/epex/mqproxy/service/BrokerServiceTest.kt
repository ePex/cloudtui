package com.github.epex.mqproxy.service

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
import org.assertj.core.api.Assertions.assertThatThrownBy
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
    ): TextMessage {
        val msg = mockk<TextMessage>()
        every { msg.jmsMessageID } returns id
        every { msg.jmsTimestamp } returns timestamp
        every { msg.text } returns body
        every { msg.jmsDeliveryMode } returns 2
        every { msg.jmsPriority } returns 4
        every { msg.jmsCorrelationID } returns null
        every { msg.jmsReplyTo } returns null
        every { msg.jmsDestination } returns mockk<jakarta.jms.Destination>(relaxed = true)
        every { msg.jmsRedelivered } returns false
        every { msg.propertyNames } returns Collections.emptyEnumeration<Any>()
        return msg
    }

    private fun stubBrowser(session: Session, queueName: String, messages: List<Message>): QueueBrowser {
        val queue = mockk<Queue>()
        val browser = mockk<QueueBrowser>()
        every { session.createQueue(queueName) } returns queue
        every { session.createBrowser(queue) } returns browser
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
        assertThat(result[0].pendingCount).isEqualTo(-1L)
        assertThat(result[0].consumerCount).isEqualTo(-1L)
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
        val msg = stubTextMessage(session, id = "ID:test-1", timestamp = 1_700_000_000_000L, body = "hello")
        stubBrowser(session, "orders", listOf(msg))

        val result = service.browseMessages("orders")

        assertThat(result).hasSize(1)
        assertThat(result[0].id).isEqualTo("ID:test-1")
        assertThat(result[0].body).isEqualTo("hello")
        assertThat(result[0].timestamp).startsWith("2023-")
    }

    @Test
    fun `browseMessages returns empty list for empty queue`() {
        val conn = mockConnection()
        val session = stubSession(conn)
        stubBrowser(session, "empty.queue", emptyList())

        val result = service.browseMessages("empty.queue")

        assertThat(result).isEmpty()
    }

    // -------------------------------------------------------------------------
    // getMessage
    // -------------------------------------------------------------------------

    @Test
    fun `getMessage returns MessageDetail for matching id`() {
        val conn = mockConnection()
        val session = stubSession(conn)
        val msg = stubTextMessage(session, id = "ID:abc-1", body = "payload")
        stubBrowser(session, "myQueue", listOf(msg))

        val result = service.getMessage("myQueue", "ID:abc-1")

        assertThat(result.id).isEqualTo("ID:abc-1")
        assertThat(result.body).isEqualTo("payload")
        assertThat(result.priority).isEqualTo(4)
    }

    @Test
    fun `getMessage throws NotFoundException when id not in queue`() {
        val conn = mockConnection()
        val session = stubSession(conn)
        val msg = stubTextMessage(session, id = "ID:other")
        stubBrowser(session, "myQueue", listOf(msg))

        assertThatThrownBy { service.getMessage("myQueue", "ID:missing") }
            .isInstanceOf(NotFoundException::class.java)
            .hasMessageContaining("ID:missing")
    }

    // -------------------------------------------------------------------------
    // deleteMessage
    // -------------------------------------------------------------------------

    @Test
    fun `deleteMessage consumes message with selector`() {
        val conn = mockConnection()
        val session = stubSession(conn)
        val queue = mockk<Queue>()
        val consumer = mockk<MessageConsumer>()
        val msg = mockk<Message>()

        every { session.createQueue("dlq") } returns queue
        every { session.createConsumer(queue, "JMSMessageID = 'ID:x-1'") } returns consumer
        every { consumer.receive(2_000) } returns msg
        every { consumer.close() } just Runs

        service.deleteMessage("dlq", "ID:x-1")

        verify { consumer.receive(2_000) }
        verify { consumer.close() }
    }

    @Test
    fun `deleteMessage throws NotFoundException when message absent`() {
        val conn = mockConnection()
        val session = stubSession(conn)
        val queue = mockk<Queue>()
        val consumer = mockk<MessageConsumer>()

        every { session.createQueue("dlq") } returns queue
        every { session.createConsumer(queue, any()) } returns consumer
        every { consumer.receive(2_000) } returns null
        every { consumer.close() } just Runs

        assertThatThrownBy { service.deleteMessage("dlq", "ID:gone") }
            .isInstanceOf(NotFoundException::class.java)
    }

    // -------------------------------------------------------------------------
    // moveMessage
    // -------------------------------------------------------------------------

    @Test
    fun `moveMessage commits transaction on success`() {
        val conn = mockConnection()
        val session = stubSession(conn, transacted = true)
        val srcQueue = mockk<Queue>()
        val dstQueue = mockk<Queue>()
        val consumer = mockk<MessageConsumer>()
        val producer = mockk<MessageProducer>()
        val msg = mockk<Message>()

        every { session.createQueue("src") } returns srcQueue
        every { session.createQueue("dst") } returns dstQueue
        every { session.createConsumer(srcQueue, "JMSMessageID = 'ID:m1'") } returns consumer
        every { session.createProducer(dstQueue) } returns producer
        every { consumer.receive(2_000) } returns msg
        every { producer.send(msg) } just Runs
        every { session.commit() } just Runs
        every { producer.close() } just Runs
        every { consumer.close() } just Runs

        service.moveMessage("src", "ID:m1", "dst")

        verify { session.commit() }
        verify { producer.send(msg) }
    }

    @Test
    fun `moveMessage rolls back and throws NotFoundException when message absent`() {
        val conn = mockConnection()
        val session = stubSession(conn, transacted = true)
        val srcQueue = mockk<Queue>()
        val consumer = mockk<MessageConsumer>()

        every { session.createQueue("src") } returns srcQueue
        every { session.createConsumer(srcQueue, any()) } returns consumer
        every { consumer.receive(2_000) } returns null
        every { session.rollback() } just Runs
        every { consumer.close() } just Runs

        assertThatThrownBy { service.moveMessage("src", "ID:gone", "dst") }
            .isInstanceOf(NotFoundException::class.java)

        verify { session.rollback() }
    }

    // -------------------------------------------------------------------------
    // moveAll
    // -------------------------------------------------------------------------

    @Test
    fun `moveAll moves all messages and returns count`() {
        val conn = mockConnection()
        val session = stubSession(conn, transacted = true)
        val srcQueue = mockk<Queue>()
        val dstQueue = mockk<Queue>()
        val consumer = mockk<MessageConsumer>()
        val producer = mockk<MessageProducer>()
        val msg1 = mockk<Message>()
        val msg2 = mockk<Message>()

        every { session.createQueue("src") } returns srcQueue
        every { session.createQueue("dst") } returns dstQueue
        every { session.createConsumer(srcQueue) } returns consumer
        every { session.createProducer(dstQueue) } returns producer
        every { consumer.receiveNoWait() } returnsMany listOf(msg1, msg2, null)
        every { producer.send(any()) } just Runs
        every { session.commit() } just Runs
        every { producer.close() } just Runs
        every { consumer.close() } just Runs

        val count = service.moveAll("src", "dst")

        assertThat(count).isEqualTo(2)
        verify(exactly = 2) { producer.send(any()) }
        verify { session.commit() }
    }

    @Test
    fun `moveAll returns zero for empty queue`() {
        val conn = mockConnection()
        val session = stubSession(conn, transacted = true)
        val srcQueue = mockk<Queue>()
        val dstQueue = mockk<Queue>()
        val consumer = mockk<MessageConsumer>()
        val producer = mockk<MessageProducer>()

        every { session.createQueue("src") } returns srcQueue
        every { session.createQueue("dst") } returns dstQueue
        every { session.createConsumer(srcQueue) } returns consumer
        every { session.createProducer(dstQueue) } returns producer
        every { consumer.receiveNoWait() } returns null
        every { session.commit() } just Runs
        every { producer.close() } just Runs
        every { consumer.close() } just Runs

        assertThat(service.moveAll("src", "dst")).isEqualTo(0)
    }

    // -------------------------------------------------------------------------
    // sendMessage
    // -------------------------------------------------------------------------

    @Test
    fun `sendMessage creates and sends TextMessage`() {
        val conn = mockConnection()
        val session = stubSession(conn)
        val queue = mockk<Queue>()
        val producer = mockk<MessageProducer>()
        val textMsg = mockk<TextMessage>()

        every { session.createQueue("outbox") } returns queue
        every { session.createProducer(queue) } returns producer
        every { session.createTextMessage("hello world") } returns textMsg
        every { producer.send(textMsg) } just Runs
        every { producer.close() } just Runs

        service.sendMessage("outbox", "hello world")

        verify { producer.send(textMsg) }
    }

    // -------------------------------------------------------------------------
    // purgeQueue
    // -------------------------------------------------------------------------

    @Test
    fun `purgeQueue drains queue and returns count`() {
        val conn = mockConnection()
        val session = stubSession(conn)
        val queue = mockk<Queue>()
        val consumer = mockk<MessageConsumer>()

        every { session.createQueue("trash") } returns queue
        every { session.createConsumer(queue) } returns consumer
        every { consumer.receiveNoWait() } returnsMany listOf(mockk(), mockk(), mockk(), null)
        every { consumer.close() } just Runs

        assertThat(service.purgeQueue("trash")).isEqualTo(3)
    }

    @Test
    fun `purgeQueue returns zero for empty queue`() {
        val conn = mockConnection()
        val session = stubSession(conn)
        val queue = mockk<Queue>()
        val consumer = mockk<MessageConsumer>()

        every { session.createQueue("trash") } returns queue
        every { session.createConsumer(queue) } returns consumer
        every { consumer.receiveNoWait() } returns null
        every { consumer.close() } just Runs

        assertThat(service.purgeQueue("trash")).isEqualTo(0)
    }
}
