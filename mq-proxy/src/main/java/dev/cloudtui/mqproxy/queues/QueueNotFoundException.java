package dev.cloudtui.mqproxy.queues;

/** The named queue isn't currently known to the broker. */
public class QueueNotFoundException extends RuntimeException {

    public QueueNotFoundException(String queueName) {
        super("queue not found: " + queueName);
    }
}
