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
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;

/**
 * Exercises {@code POST /queues/{name}/purge} against the real local
 * embedded broker.
 */
@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("local")
class QueuesControllerPurgeTest {

    @Autowired
    private MockMvc mockMvc;

    @Autowired
    private ConnectionFactory connectionFactory;

    @Test
    void purgeRemovesAllMessages() throws Exception {
        String queueName = "purge-test-" + System.nanoTime();
        try (Connection connection = connectionFactory.createConnection()) {
            connection.start();
            Session session = connection.createSession(false, Session.AUTO_ACKNOWLEDGE);
            Queue queue = session.createQueue(queueName);
            MessageProducer producer = session.createProducer(queue);
            producer.send(session.createTextMessage("one"));
            producer.send(session.createTextMessage("two"));

            awaitMessagesVisible(queueName);

            mockMvc.perform(post("/queues/{name}/purge", queueName).with(httpBasic("admin", "admin")))
                .andExpect(status().isNoContent());

            mockMvc.perform(get("/queues/{name}/messages", queueName).with(httpBasic("admin", "admin")))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.length()").value(0));
        }
    }

    @Test
    void purgeOfUnknownQueueReturnsNotFound() throws Exception {
        mockMvc.perform(post("/queues/{name}/purge", "does-not-exist-" + System.nanoTime())
                .with(httpBasic("admin", "admin")))
            .andExpect(status().isNotFound());
    }

    private void awaitMessagesVisible(String queueName) throws Exception {
        for (int attempt = 0; attempt < 10; attempt++) {
            var result = mockMvc.perform(get("/queues/{name}/messages", queueName)
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
