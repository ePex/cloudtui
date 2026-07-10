package dev.cloudtui.mqproxy.queues;

import dev.cloudtui.mqproxy.generated.model.Message;
import dev.cloudtui.mqproxy.generated.model.QueueSummary;
import jakarta.annotation.PreDestroy;
import jakarta.jms.Connection;
import jakarta.jms.ConnectionFactory;
import jakarta.jms.Destination;
import jakarta.jms.JMSException;
import jakarta.jms.MapMessage;
import jakarta.jms.MessageConsumer;
import jakarta.jms.MessageProducer;
import jakarta.jms.Queue;
import jakarta.jms.QueueBrowser;
import jakarta.jms.Session;
import jakarta.jms.TemporaryQueue;
import jakarta.jms.TextMessage;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.Enumeration;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import org.apache.activemq.ActiveMQConnection;
import org.apache.activemq.advisory.DestinationSource;
import org.apache.activemq.command.ActiveMQQueue;
import org.springframework.stereotype.Service;

/**
 * Queue operations over JMS/OpenWire. Keeps one long-lived connection (and
 * its {@link DestinationSource}, which populates asynchronously from
 * broker advisories) for the service's lifetime, rather than reconnecting
 * per request — reconnecting per call would mean re-paying the advisory
 * propagation delay on every single list(). The connection is established
 * lazily, on first use, rather than at bean-construction time: the broker
 * may be briefly unreachable when mq-proxy starts, and that shouldn't
 * block application startup or fail unrelated requests — only queue
 * operations actually need it.
 */
@Service
public class QueueService {

    private static final long STATS_REPLY_TIMEOUT_MS = 3000;
    private static final long DRAIN_TIMEOUT_MS = 500;

    private final ConnectionFactory connectionFactory;
    private volatile Connection connection;
    private volatile DestinationSource destinationSource;

    public QueueService(ConnectionFactory connectionFactory) {
        this.connectionFactory = connectionFactory;
    }

    @PreDestroy
    void shutdown() throws JMSException {
        if (connection != null) {
            connection.close();
        }
    }

    /** Lists queues known to the broker, each with its current statistics. */
    public List<QueueSummary> list() {
        connect();
        try (Session session = connection.createSession(false, Session.AUTO_ACKNOWLEDGE)) {
            List<QueueSummary> summaries = new ArrayList<>();
            for (ActiveMQQueue queue : destinationSource.getQueues()) {
                summaries.add(statsFor(session, queue.getQueueName()));
            }
            summaries.sort(Comparator.comparing(QueueSummary::getName));
            return summaries;
        } catch (JMSException e) {
            throw new QueueOperationException("listing queues", e);
        }
    }

    /** Browses (without consuming) up to {@code limit} messages on a queue. */
    public List<Message> browse(String queueName, int limit) {
        requireQueueExists(queueName);
        try (Session session = connection.createSession(false, Session.AUTO_ACKNOWLEDGE)) {
            Queue queue = session.createQueue(queueName);
            QueueBrowser browser = session.createBrowser(queue);
            List<Message> messages = new ArrayList<>();
            Enumeration<?> enumeration = browser.getEnumeration();
            while (enumeration.hasMoreElements() && messages.size() < limit) {
                messages.add(toApiMessage((jakarta.jms.Message) enumeration.nextElement()));
            }
            browser.close();
            return messages;
        } catch (JMSException e) {
            throw new QueueOperationException("browsing queue " + queueName, e);
        }
    }

    private Message toApiMessage(jakarta.jms.Message jmsMessage) throws JMSException {
        String body = jmsMessage instanceof TextMessage text ? text.getText() : "";
        Map<String, String> properties = new HashMap<>();
        Enumeration<?> names = jmsMessage.getPropertyNames();
        while (names.hasMoreElements()) {
            String name = (String) names.nextElement();
            properties.put(name, String.valueOf(jmsMessage.getObjectProperty(name)));
        }
        return new Message(jmsMessage.getJMSMessageID(), body).properties(properties);
    }

    /**
     * Sends a message to a queue. Unlike browse/purge/move, this
     * deliberately does not require the queue to already exist —
     * publishing to a brand-new queue name is normal JMS/ActiveMQ usage
     * and implicitly creates it, and forbidding that here would surprise
     * callers.
     */
    public void send(String queueName, String body, Map<String, String> properties) {
        connect();
        try (Session session = connection.createSession(false, Session.AUTO_ACKNOWLEDGE)) {
            Queue queue = session.createQueue(queueName);
            try (MessageProducer producer = session.createProducer(queue)) {
                TextMessage message = session.createTextMessage(body);
                if (properties != null) {
                    for (Map.Entry<String, String> entry : properties.entrySet()) {
                        message.setStringProperty(entry.getKey(), entry.getValue());
                    }
                }
                producer.send(message);
            }
        } catch (JMSException e) {
            throw new QueueOperationException("sending to queue " + queueName, e);
        }
    }

    /** Removes all messages currently on a queue, atomically. */
    public void purge(String queueName) {
        requireQueueExists(queueName);
        try (Session session = connection.createSession(true, Session.SESSION_TRANSACTED)) {
            Queue queue = session.createQueue(queueName);
            try (MessageConsumer consumer = session.createConsumer(queue)) {
                while (consumer.receive(DRAIN_TIMEOUT_MS) != null) {
                    // drain until the queue reports empty
                }
                session.commit();
            }
        } catch (JMSException e) {
            throw new QueueOperationException("purging queue " + queueName, e);
        }
    }

    /**
     * Moves up to {@code maxMessages} (or all, if {@code null}) messages
     * from one queue to another, atomically. The source must already
     * exist; the target follows send()'s auto-create semantics.
     */
    public void move(String sourceQueueName, String targetQueueName, Integer maxMessages) {
        requireQueueExists(sourceQueueName);
        int limit = maxMessages != null ? maxMessages : Integer.MAX_VALUE;
        try (Session session = connection.createSession(true, Session.SESSION_TRANSACTED)) {
            Queue source = session.createQueue(sourceQueueName);
            Queue target = session.createQueue(targetQueueName);
            try (MessageConsumer consumer = session.createConsumer(source);
                    MessageProducer producer = session.createProducer(target)) {
                int moved = 0;
                jakarta.jms.Message message;
                while (moved < limit && (message = consumer.receive(DRAIN_TIMEOUT_MS)) != null) {
                    producer.send(message);
                    moved++;
                }
                session.commit();
            }
        } catch (JMSException e) {
            throw new QueueOperationException(
                "moving messages from " + sourceQueueName + " to " + targetQueueName, e);
        }
    }

    /** Throws {@link QueueNotFoundException} unless the broker currently knows this queue. */
    private void requireQueueExists(String queueName) {
        connect();
        boolean exists = destinationSource.getQueues().stream().anyMatch(q -> {
            try {
                return queueName.equals(q.getQueueName());
            } catch (JMSException e) {
                throw new QueueOperationException("reading queue name", e);
            }
        });
        if (!exists) {
            throw new QueueNotFoundException(queueName);
        }
    }

    private synchronized void connect() {
        if (connection != null) {
            return;
        }
        try {
            connection = connectionFactory.createConnection();
            connection.start();
            destinationSource = ((ActiveMQConnection) connection).getDestinationSource();
            destinationSource.start();
        } catch (JMSException e) {
            connection = null;
            throw new QueueOperationException("connecting to broker", e);
        }
    }

    /**
     * Queries a queue's pending/consumer counts via ActiveMQ's built-in
     * per-destination statistics mechanism (a client-side technique, not a
     * broker-side plugin call from here — but it does require the broker
     * to have {@code StatisticsBrokerPlugin} enabled, which Amazon MQ does
     * by default on managed brokers and {@code LocalBrokerConfig} does
     * explicitly for the local embedded one).
     */
    private QueueSummary statsFor(Session session, String queueName) throws JMSException {
        Destination statsDestination = session.createQueue("ActiveMQ.Statistics.Destination." + queueName);
        TemporaryQueue replyTo = session.createTemporaryQueue();
        try (MessageConsumer replyConsumer = session.createConsumer(replyTo);
                MessageProducer producer = session.createProducer(statsDestination)) {
            jakarta.jms.Message request = session.createMessage();
            request.setJMSReplyTo(replyTo);
            producer.send(request);

            jakarta.jms.Message reply = replyConsumer.receive(STATS_REPLY_TIMEOUT_MS);
            if (!(reply instanceof MapMessage stats)) {
                throw new QueueOperationException("no statistics reply for queue " + queueName, null);
            }
            return new QueueSummary(queueName, stats.getLong("size"), stats.getLong("consumerCount"));
        } finally {
            replyTo.delete();
        }
    }
}
