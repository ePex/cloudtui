package com.github.epex.mqproxy.service

import com.github.epex.mqproxy.api.model.MessageDetail
import com.github.epex.mqproxy.api.model.MessageSummary
import com.github.epex.mqproxy.api.model.QueueSummary
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
                        pendingCount = stats?.getOrDefault("size", -1L) ?: -1L,
                        consumerCount = stats?.getOrDefault("consumerCount", -1L) ?: -1L,
                        enqueueCount = stats?.getOrDefault("enqueueCount", -1L) ?: -1L,
                        dequeueCount = stats?.getOrDefault("dequeueCount", -1L) ?: -1L,
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

    fun browseMessages(queueName: String): List<MessageSummary> {
        log.info("broker: browseMessages queue={}", queueName)
        val connection = connectionFactory.createConnection()
        connection.start()
        return try {
            val session = connection.createSession(false, Session.AUTO_ACKNOWLEDGE)
            val browser = session.createBrowser(session.createQueue(queueName))
            val messages = mutableListOf<MessageSummary>()
            val enum = browser.enumeration
            while (enum.hasMoreElements()) {
                val msg = enum.nextElement() as? jakarta.jms.Message ?: continue
                messages += msg.toSummary()
            }
            browser.close()
            messages
        } finally {
            connection.close()
        }
    }

    fun getMessage(queueName: String, messageId: String): MessageDetail {
        log.info("broker: getMessage queue={} id={}", queueName, messageId)
        val connection = connectionFactory.createConnection()
        connection.start()
        return try {
            val session = connection.createSession(false, Session.AUTO_ACKNOWLEDGE)
            val browser = session.createBrowser(session.createQueue(queueName))
            val enum = browser.enumeration
            while (enum.hasMoreElements()) {
                val msg = enum.nextElement() as? jakarta.jms.Message ?: continue
                if (msg.jmsMessageID == messageId) {
                    browser.close()
                    return msg.toDetail()
                }
            }
            browser.close()
            throw NotFoundException("Message '$messageId' not found in queue '$queueName'")
        } finally {
            connection.close()
        }
    }

    // -------------------------------------------------------------------------
    // Message write operations
    // -------------------------------------------------------------------------

    fun purgeQueue(queueName: String): Int {
        log.info("broker: purgeQueue queue={}", queueName)
        val connection = connectionFactory.createConnection()
        connection.start()
        try {
            val session = connection.createSession(false, Session.AUTO_ACKNOWLEDGE)
            val consumer = session.createConsumer(session.createQueue(queueName))
            var count = 0
            while (true) {
                consumer.receiveNoWait() ?: break
                count++
            }
            consumer.close()
            return count
        } finally {
            connection.close()
        }
    }

    fun sendMessage(queueName: String, body: String) {
        log.info("broker: sendMessage queue={} bodyLength={}", queueName, body.length)
        val connection = connectionFactory.createConnection()
        connection.start()
        try {
            val session = connection.createSession(false, Session.AUTO_ACKNOWLEDGE)
            val producer = session.createProducer(session.createQueue(queueName))
            producer.send(session.createTextMessage(body))
            producer.close()
        } finally {
            connection.close()
        }
    }

    fun moveAll(queueName: String, destination: String): Int {
        log.info("broker: moveAll queue={} to={}", queueName, destination)
        val connection = connectionFactory.createConnection()
        connection.start()
        try {
            val session = connection.createSession(true, Session.SESSION_TRANSACTED)
            val consumer = session.createConsumer(session.createQueue(queueName))
            val producer = session.createProducer(session.createQueue(destination))
            var count = 0
            while (true) {
                val msg = consumer.receiveNoWait() ?: break
                producer.send(msg)
                count++
            }
            session.commit()
            producer.close()
            consumer.close()
            return count
        } finally {
            connection.close()
        }
    }

    fun moveMessage(queueName: String, messageId: String, destination: String) {
        log.info("broker: moveMessage queue={} id={} to={}", queueName, messageId, destination)
        val connection = connectionFactory.createConnection()
        connection.start()
        try {
            val session = connection.createSession(true, Session.SESSION_TRANSACTED)
            val consumer = session.createConsumer(session.createQueue(queueName), jmsIdSelector(messageId))
            val msg = consumer.receive(2_000)
            if (msg == null) {
                session.rollback()
                throw NotFoundException("Message '$messageId' not found in queue '$queueName'")
            }
            val producer = session.createProducer(session.createQueue(destination))
            producer.send(msg)
            session.commit()
            producer.close()
            consumer.close()
        } finally {
            connection.close()
        }
    }

    fun deleteMessage(queueName: String, messageId: String) {
        log.info("broker: deleteMessage queue={} id={}", queueName, messageId)
        val connection = connectionFactory.createConnection()
        connection.start()
        try {
            val session = connection.createSession(false, Session.AUTO_ACKNOWLEDGE)
            val selector = jmsIdSelector(messageId)
            val consumer = session.createConsumer(session.createQueue(queueName), selector)
            val msg = consumer.receive(2_000)
            consumer.close()
            if (msg == null) throw NotFoundException("Message '$messageId' not found in queue '$queueName'")
        } finally {
            connection.close()
        }
    }

    // -------------------------------------------------------------------------
    // Private helpers
    // -------------------------------------------------------------------------

    /**
     * Builds a JMS message selector that matches a single message by its ID.
     * Single quotes inside the ID are escaped by doubling them, as required
     * by the JMS selector grammar.
     */
    private fun jmsIdSelector(messageId: String): String {
        val escaped = messageId.replace("'", "''")
        return "JMSMessageID = '$escaped'"
    }

    private fun jakarta.jms.Message.toSummary() = MessageSummary(
        id = jmsMessageID ?: "",
        timestamp = epochMillisToIso(jmsTimestamp),
        body = (this as? TextMessage)?.text,
        properties = stringProperties(),
    )

    private fun jakarta.jms.Message.toDetail() = MessageDetail(
        id = jmsMessageID ?: "",
        timestamp = epochMillisToIso(jmsTimestamp),
        body = (this as? TextMessage)?.text,
        deliveryMode = jmsDeliveryMode,
        priority = jmsPriority,
        correlationId = jmsCorrelationID,
        replyTo = jmsReplyTo?.toString(),
        destination = jmsDestination?.toString() ?: "",
        redelivered = jmsRedelivered,
        properties = stringProperties(),
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
            )
        } finally {
            producer.close()
            consumer.close()
            runCatching { replyTo.delete() }
        }
    }
}
