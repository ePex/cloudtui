package dev.cloudtui.mqproxy.queues;

import static org.springframework.security.test.web.servlet.request.SecurityMockMvcRequestPostProcessors.httpBasic;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
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
 * Exercises {@code GET /queues/{name}/messages} against the real local
 * embedded broker.
 */
@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("local")
class QueuesControllerBrowseTest {

    @Autowired
    private MockMvc mockMvc;

    @Autowired
    private ConnectionFactory connectionFactory;

    @Test
    void browsesMessagesWithoutConsumingThem() throws Exception {
        String queueName = "browse-test-" + System.nanoTime();
        try (Connection connection = connectionFactory.createConnection()) {
            connection.start();
            Session session = connection.createSession(false, Session.AUTO_ACKNOWLEDGE);
            Queue queue = session.createQueue(queueName);
            MessageProducer producer = session.createProducer(queue);
            producer.send(session.createTextMessage("first"));
            producer.send(session.createTextMessage("second"));

            awaitQueueVisible(queueName);

            mockMvc.perform(get("/queues/{name}/messages", queueName).with(httpBasic("admin", "admin")))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.length()").value(2))
                .andExpect(jsonPath("$[0].body").value("first"))
                .andExpect(jsonPath("$[1].body").value("second"));

            // Browsing must not consume: the messages are still there.
            mockMvc.perform(get("/queues/{name}/messages", queueName).with(httpBasic("admin", "admin")))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.length()").value(2));
        }
    }

    @Test
    void returnsNotFoundForAnUnknownQueue() throws Exception {
        mockMvc.perform(get("/queues/{name}/messages", "does-not-exist-" + System.nanoTime())
                .with(httpBasic("admin", "admin")))
            .andExpect(status().isNotFound());
    }

    private void awaitQueueVisible(String queueName) throws Exception {
        for (int attempt = 0; attempt < 10; attempt++) {
            var result = mockMvc.perform(get("/queues").with(httpBasic("admin", "admin"))).andReturn();
            if (result.getResponse().getContentAsString().contains(queueName)) {
                return;
            }
            Thread.sleep(300);
        }
        throw new AssertionError(queueName + " never appeared in GET /queues");
    }
}
