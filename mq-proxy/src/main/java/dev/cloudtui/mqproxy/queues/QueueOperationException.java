package dev.cloudtui.mqproxy.queues;

/** A JMS/broker-level failure while performing a queue operation. */
public class QueueOperationException extends RuntimeException {

    public QueueOperationException(String message, Throwable cause) {
        super(message, cause);
    }
}
