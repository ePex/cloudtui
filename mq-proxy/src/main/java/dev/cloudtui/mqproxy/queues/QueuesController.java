package dev.cloudtui.mqproxy.queues;

import dev.cloudtui.mqproxy.generated.api.QueuesApi;
import dev.cloudtui.mqproxy.generated.model.Message;
import dev.cloudtui.mqproxy.generated.model.MoveMessagesRequest;
import dev.cloudtui.mqproxy.generated.model.QueueSummary;
import dev.cloudtui.mqproxy.generated.model.SendMessageRequest;
import java.util.List;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.RestController;

/**
 * Implements the generated {@link QueuesApi} contract. Send/purge/move
 * are filled in by later tasks; until then they fall back to the
 * generated stub (501), which also proves the generated code compiles and
 * wires into the Spring context.
 */
@RestController
public class QueuesController implements QueuesApi {

    private final QueueService queueService;

    public QueuesController(QueueService queueService) {
        this.queueService = queueService;
    }

    @Override
    public ResponseEntity<List<QueueSummary>> listQueues() {
        return ResponseEntity.ok(queueService.list());
    }

    @Override
    public ResponseEntity<List<Message>> browseMessages(String name, Integer limit) {
        return ResponseEntity.ok(queueService.browse(name, limit));
    }

    @Override
    public ResponseEntity<Void> sendMessage(String name, SendMessageRequest sendMessageRequest) {
        queueService.send(name, sendMessageRequest.getBody(), sendMessageRequest.getProperties());
        return ResponseEntity.status(201).build();
    }

    @Override
    public ResponseEntity<Void> purgeQueue(String name) {
        queueService.purge(name);
        return ResponseEntity.noContent().build();
    }

    @Override
    public ResponseEntity<Void> moveMessages(String name, MoveMessagesRequest moveMessagesRequest) {
        queueService.move(name, moveMessagesRequest.getTargetQueue(), moveMessagesRequest.getMaxMessages());
        return ResponseEntity.noContent().build();
    }
}
