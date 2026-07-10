package dev.cloudtui.mqproxy.queues;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.security.test.web.servlet.request.SecurityMockMvcRequestPostProcessors.httpBasic;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.fasterxml.jackson.databind.ObjectMapper;
import jakarta.jms.Connection;
import jakarta.jms.ConnectionFactory;
import jakarta.jms.MessageProducer;
import jakarta.jms.Queue;
import jakarta.jms.Session;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.webmvc.test.autoconfigure.AutoConfigureMockMvc;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.MvcResult;

/**
 * Exercises {@code GET /queues} against the real local embedded broker:
 * sends messages to a fresh queue, then confirms it shows up with correct
 * statistics. Polls, rather than asserting once immediately, since
 * ActiveMQ's {@code DestinationSource} populates from broker advisories
 * asynchronously.
 */
@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("local")
class QueuesControllerListTest {

    @Autowired
    private MockMvc mockMvc;

    @Autowired
    private ConnectionFactory connectionFactory;

    private final ObjectMapper objectMapper = new ObjectMapper();

    @Test
    void listsQueuesWithStatistics() throws Exception {
        String queueName = "list-test-" + System.nanoTime();
        try (Connection connection = connectionFactory.createConnection()) {
            connection.start();
            Session session = connection.createSession(false, Session.AUTO_ACKNOWLEDGE);
            Queue queue = session.createQueue(queueName);
            MessageProducer producer = session.createProducer(queue);
            producer.send(session.createTextMessage("one"));
            producer.send(session.createTextMessage("two"));

            Map<String, Object> found = awaitQueue(queueName);

            assertThat(((Number) found.get("pendingCount")).longValue()).isEqualTo(2L);
            assertThat(((Number) found.get("consumerCount")).longValue()).isEqualTo(0L);
        }
    }

    @SuppressWarnings("unchecked")
    private Map<String, Object> awaitQueue(String queueName) throws Exception {
        for (int attempt = 0; attempt < 10; attempt++) {
            MvcResult result = mockMvc.perform(get("/queues").with(httpBasic("admin", "admin")))
                .andExpect(status().isOk())
                .andReturn();
            List<Map<String, Object>> queues =
                objectMapper.readValue(result.getResponse().getContentAsString(), List.class);
            var match = queues.stream().filter(q -> queueName.equals(q.get("name"))).findFirst();
            if (match.isPresent()) {
                return match.get();
            }
            Thread.sleep(300);
        }
        throw new AssertionError(queueName + " never appeared in GET /queues");
    }
}
