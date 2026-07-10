package dev.cloudtui.mqproxy.queues;

import static org.springframework.security.test.web.servlet.request.SecurityMockMvcRequestPostProcessors.httpBasic;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.webmvc.test.autoconfigure.AutoConfigureMockMvc;
import org.springframework.http.MediaType;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;

/**
 * Exercises {@code POST /queues/{name}/messages} against the real local
 * embedded broker — including sending to a queue name that doesn't exist
 * yet, which must succeed (JMS/ActiveMQ auto-creates it) rather than 404.
 */
@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("local")
class QueuesControllerSendTest {

    @Autowired
    private MockMvc mockMvc;

    @Test
    void sendingCreatesTheQueueAndTheMessageIsBrowsable() throws Exception {
        String queueName = "send-test-" + System.nanoTime();

        mockMvc.perform(post("/queues/{name}/messages", queueName)
                .with(httpBasic("admin", "admin"))
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"body\":\"hello there\",\"properties\":{\"kind\":\"greeting\"}}"))
            .andExpect(status().isCreated());

        awaitMessageBrowsable(queueName);
    }

    private void awaitMessageBrowsable(String queueName) throws Exception {
        for (int attempt = 0; attempt < 10; attempt++) {
            var result = mockMvc.perform(get("/queues/{name}/messages", queueName)
                    .with(httpBasic("admin", "admin")))
                .andReturn();
            String body = result.getResponse().getContentAsString();
            if (body.contains("hello there")) {
                mockMvc.perform(get("/queues/{name}/messages", queueName).with(httpBasic("admin", "admin")))
                    .andExpect(jsonPath("$[0].body").value("hello there"))
                    .andExpect(jsonPath("$[0].properties.kind").value("greeting"));
                return;
            }
            Thread.sleep(300);
        }
        throw new AssertionError("message never became browsable on " + queueName);
    }
}
