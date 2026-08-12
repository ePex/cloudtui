package com.github.epex.mqproxy.service

import com.github.epex.mqproxy.api.model.DeletedMessageDto
import com.github.epex.mqproxy.api.model.MessageSummary
import com.github.epex.mqproxy.api.model.MovedMessageDto
import com.github.epex.mqproxy.api.model.QueueMessageFilter
import com.github.epex.mqproxy.api.model.QueueSummary
import com.github.epex.mqproxy.api.model.SendMessageRequest
import jakarta.jms.MapMessage
import jakarta.jms.Session
import jakarta.jms.TextMessage
import org.apache.activemq.ActiveMQConnection
import org.apache.activemq.ActiveMQConnectionFactory
import org.slf4j.LoggerFactory
import org.springframework.stereotype.Service
import java.time.Instant
import java.time.ZoneOffset
import java.time.format.DateTimeFormatter

@Service
class BrokerService(private val connectionFactory: ActiveMQConnectionFactory) {

    private val log = LoggerFactory.getLogger(BrokerService::class.java)

    // -------------------------------------------------------------------------
    // Queue listing
    // -------------------------------------------------------------------------

    fun listQueues(): List<QueueSummary> {
        log.info("broker: listQueues")
        val connection = connectionFactory.createConnection() as ActiveMQConnection
        connection.start()
        return try {
            val session = connection.createSession(false, Session.AUTO_ACKNOWLEDGE)
            // Give the advisory-based DestinationSource a moment to populate
            // after the connection starts.
            Thread.sleep(200)
            connection.destinationSource.queues
                .filter { !it.physicalName.startsWith("ActiveMQ.") }
                .map { queue ->
                    val stats = fetchStats(session, queue.physicalName)
                    QueueSummary(
                        name = queue.physicalName,
                        messageCount = stats?.getOrDefault("size", -1L) ?: -1L,
                        consumerCount = stats?.getOrDefault("consumerCount", -1L) ?: -1L,
                        enqueuedCount = stats?.getOrDefault("enqueueCount", -1L) ?: -1L,
                        dequeuedCount = stats?.getOrDefault("dequeueCount", -1L) ?: -1L,
                        producerCount = stats?.getOrDefault("producerCount", -1L) ?: -1L,
                    )
                }
                .sortedBy { it.name }
        } finally {
            connection.close()
        }
    }

    // -------------------------------------------------------------------------
    // Message browsing
    // -------------------------------------------------------------------------

    fun browseMessages(queueName: String, filter: QueueMessageFilter): List<MessageSummary> {
        log.info("broker: browseMessages queue={} filter={}", queueName, filter)
        val connection = connectionFactory.createConnection()
        connection.start()
        return try {
            val session = connection.createSession(false, Session.AUTO_ACKNOWLEDGE)
            val browser = session.createBrowser(session.createQueue(queueName), filter.toSelector())
            val messages = mutableListOf<MessageSummary>()
            val enum = browser.enumeration
            while (enum.hasMoreElements()) {
                val msg = enum.nextElement() as? jakarta.jms.Message ?: continue
                messages += msg.toSummary(queueName)
            }
            browser.close()
            messages
        } finally {
            connection.close()
        }
    }

    // -------------------------------------------------------------------------
    // Message write operations
    // -------------------------------------------------------------------------

    fun sendMessage(request: SendMessageRequest): String {
        log.info(
            "broker: sendMessage queue={} jmsType={} bodyLength={}",
            request.targetQueue, request.jmsType, request.body.length,
        )
        val connection = connectionFactory.createConnection()
        connection.start()
        try {
            val session = connection.createSession(false, Session.AUTO_ACKNOWLEDGE)
            val producer = session.createProducer(session.createQueue(request.targetQueue))
            val message = session.createTextMessage(request.body)
            message.jmsType = request.jmsType
            request.correlationId?.let { message.jmsCorrelationID = it }
            request.groupId?.let { message.setStringProperty("JMSXGroupID", it) }
            request.headers?.forEach { (key, value) -> message.setStringProperty(key, value) }
            producer.send(message)
            producer.close()
            return message.jmsMessageID
        } finally {
            connection.close()
        }
    }

    fun deleteMessages(sourceQueue: String, filter: QueueMessageFilter): List<DeletedMessageDto> {
        log.info("broker: deleteMessages queue={} filter={}", sourceQueue, filter)
        val connection = connectionFactory.createConnection()
        connection.start()
        try {
            val session = connection.createSession(false, Session.AUTO_ACKNOWLEDGE)
            val consumer = session.createConsumer(session.createQueue(sourceQueue), filter.toSelector())
            val deleted = mutableListOf<DeletedMessageDto>()
            while (filter.maxCount == null || deleted.size < filter.maxCount) {
                val msg = consumer.receiveNoWait() ?: break
                deleted += DeletedMessageDto(messageId = msg.jmsMessageID ?: "")
            }
            consumer.close()
            return deleted
        } finally {
            connection.close()
        }
    }

    fun moveMessages(sourceQueue: String, targetQueue: String, filter: QueueMessageFilter): List<MovedMessageDto> {
        log.info("broker: moveMessages queue={} to={} filter={}", sourceQueue, targetQueue, filter)
        val connection = connectionFactory.createConnection()
        connection.start()
        try {
            val session = connection.createSession(true, Session.SESSION_TRANSACTED)
            val consumer = session.createConsumer(session.createQueue(sourceQueue), filter.toSelector())
            val producer = session.createProducer(session.createQueue(targetQueue))
            val moved = mutableListOf<MovedMessageDto>()
            while (filter.maxCount == null || moved.size < filter.maxCount) {
                val msg = consumer.receiveNoWait() ?: break
                producer.send(msg)
                moved += MovedMessageDto(messageId = msg.jmsMessageID ?: "")
            }
            session.commit()
            producer.close()
            consumer.close()
            return moved
        } finally {
            connection.close()
        }
    }

    // -------------------------------------------------------------------------
    // Private helpers
    // -------------------------------------------------------------------------

    /**
     * Builds a JMS message selector from this filter's fields, or `null` if
     * none are set (matching everything — the "purge"/"move all" case).
     * `JMSType`, `JMSMessageID`, and `JMSTimestamp` are all selectable per
     * the JMS spec, so criteria can be pushed down to the broker instead of
     * fetched-then-filtered client-side. `maxCount` isn't expressible in a
     * selector and is enforced by the caller's receive loop instead.
     */
    private fun QueueMessageFilter.toSelector(): String? {
        val clauses = mutableListOf<String>()
        jmsType?.let { clauses += "JMSType = '${it.replace("'", "''")}'" }
        messageId?.let { clauses += "JMSMessageID = '${it.replace("'", "''")}'" }
        fromDate?.let { clauses += "JMSTimestamp >= ${Instant.parse(it).toEpochMilli()}" }
        toDate?.let { clauses += "JMSTimestamp <= ${Instant.parse(it).toEpochMilli()}" }
        return clauses.takeIf { it.isNotEmpty() }?.joinToString(" AND ")
    }

    private fun jakarta.jms.Message.toSummary(sourceQueue: String) = MessageSummary(
        sourceQueue = sourceQueue,
        messageId = jmsMessageID ?: "",
        jmsType = jmsType ?: "",
        body = (this as? TextMessage)?.text,
        timestamp = epochMillisToIso(jmsTimestamp),
        headers = stringProperties(),
    )

    private fun jakarta.jms.Message.stringProperties(): Map<String, String> {
        val result = mutableMapOf<String, String>()
        val names = propertyNames
        while (names.hasMoreElements()) {
            val key = names.nextElement().toString()
            result[key] = getStringProperty(key) ?: getObjectProperty(key)?.toString() ?: ""
        }
        return result
    }

    private fun epochMillisToIso(millis: Long): String =
        DateTimeFormatter.ISO_INSTANT.format(Instant.ofEpochMilli(millis).atOffset(ZoneOffset.UTC))

    /**
     * Fetches queue statistics via the ActiveMQ Statistics Plugin.
     * Sends an empty JMS message to `ActiveMQ.Statistics.Destination.<name>`
     * and reads the reply properties. Returns null if the plugin is not
     * enabled on the broker or the reply times out.
     */
    private fun fetchStats(session: Session, queueName: String): Map<String, Long>? {
        val replyTo = session.createTemporaryQueue()
        val consumer = session.createConsumer(replyTo)
        val producer = session.createProducer(
            session.createQueue("ActiveMQ.Statistics.Destination.$queueName")
        )
        return try {
            val request = session.createMessage()
            request.jmsReplyTo = replyTo
            producer.send(request)
            val reply = consumer.receive(3_000)
            if (reply == null) {
                log.warn("Stats plugin timeout for queue '{}' — is statisticsBrokerPlugin enabled and broker restarted?", queueName)
                return null
            }
            if (reply !is MapMessage) {
                log.warn("Stats plugin reply for queue '{}' is not a MapMessage (got {})", queueName, reply::class.simpleName)
                return null
            }
            mapOf(
                "size" to runCatching { reply.getLong("size") }.getOrDefault(-1L),
                "consumerCount" to runCatching { reply.getLong("consumerCount") }.getOrDefault(-1L),
                "enqueueCount" to runCatching { reply.getLong("enqueueCount") }.getOrDefault(-1L),
                "dequeueCount" to runCatching { reply.getLong("dequeueCount") }.getOrDefault(-1L),
                "producerCount" to runCatching { reply.getLong("producerCount") }.getOrDefault(-1L),
            )
        } finally {
            producer.close()
            consumer.close()
            runCatching { replyTo.delete() }
        }
    }
}
