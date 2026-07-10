package dev.cloudtui.mqproxy.queues;

import static org.springframework.security.test.web.servlet.request.SecurityMockMvcRequestPostProcessors.httpBasic;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import jakarta.jms.Connection;
import jakarta.jms.ConnectionFactory;
import jakarta.jms.MessageProducer;
import jakarta.jms.Queue;
import jakarta.jms.Session;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.webmvc.test.autoconfigure.AutoConfigureMockMvc;
import org.springframework.http.MediaType;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.MvcResult;

/**
 * Exercises {@code POST /queues/{name}/move} against the real local
 * embedded broker.
 */
@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("local")
class QueuesControllerMoveTest {

    @Autowired
    private MockMvc mockMvc;

    @Autowired
    private ConnectionFactory connectionFactory;

    @Test
    void moveTransfersMessagesToTheTargetQueue() throws Exception {
        String sourceQueue = "move-source-" + System.nanoTime();
        String targetQueue = "move-target-" + System.nanoTime();
        try (Connection connection = connectionFactory.createConnection()) {
            connection.start();
            Session session = connection.createSession(false, Session.AUTO_ACKNOWLEDGE);
            Queue source = session.createQueue(sourceQueue);
            MessageProducer producer = session.createProducer(source);
            producer.send(session.createTextMessage("one"));
            producer.send(session.createTextMessage("two"));

            awaitMessagesVisible(sourceQueue);

            mockMvc.perform(post("/queues/{name}/move", sourceQueue)
                    .with(httpBasic("admin", "admin"))
                    .contentType(MediaType.APPLICATION_JSON)
                    .content("{\"targetQueue\":\"" + targetQueue + "\"}"))
                .andExpect(status().isNoContent());

            mockMvc.perform(get("/queues/{name}/messages", sourceQueue).with(httpBasic("admin", "admin")))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.length()").value(0));

            awaitMessagesVisible(targetQueue);
            mockMvc.perform(get("/queues/{name}/messages", targetQueue).with(httpBasic("admin", "admin")))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.length()").value(2));
        }
    }

    @Test
    void moveOfUnknownSourceQueueReturnsNotFound() throws Exception {
        mockMvc.perform(post("/queues/{name}/move", "does-not-exist-" + System.nanoTime())
                .with(httpBasic("admin", "admin"))
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"targetQueue\":\"whatever\"}"))
            .andExpect(status().isNotFound());
    }

    private void awaitMessagesVisible(String queueName) throws Exception {
        for (int attempt = 0; attempt < 10; attempt++) {
            MvcResult result = mockMvc.perform(get("/queues/{name}/messages", queueName)
                    .with(httpBasic("admin", "admin")))
                .andReturn();
            if (result.getResponse().getStatus() == 200
                    && result.getResponse().getContentAsString().contains("\"body\"")) {
                return;
            }
            Thread.sleep(300);
        }
        throw new AssertionError(queueName + " messages never became visible");
    }
}
