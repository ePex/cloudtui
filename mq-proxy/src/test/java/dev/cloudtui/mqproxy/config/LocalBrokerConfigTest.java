package dev.cloudtui.mqproxy.config;

import static org.assertj.core.api.Assertions.assertThat;

import jakarta.jms.Connection;
import jakarta.jms.ConnectionFactory;
import jakarta.jms.Session;
import jakarta.jms.TemporaryQueue;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.webmvc.test.autoconfigure.AutoConfigureMockMvc;
import org.springframework.test.context.ActiveProfiles;

/**
 * Confirms the "local" profile's embedded broker actually starts and a
 * JMS connection can be opened against it via the shared connection
 * factory — proves the local, Docker-free dev stack works end to end.
 *
 * <p>Shares its {@code @SpringBootTest}/{@code @AutoConfigureMockMvc}/
 * {@code @ActiveProfiles("local")} signature with other local-profile
 * tests (e.g. {@link dev.cloudtui.mqproxy.config.SecurityConfigTest}) on
 * purpose: Spring's test context caching then reuses a single context —
 * and a single embedded broker bound to a single port — across all of
 * them, instead of each test class starting its own and clashing.
 */
@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("local")
class LocalBrokerConfigTest {

    @Autowired
    private ConnectionFactory connectionFactory;

    @Test
    void connectsToTheEmbeddedBroker() throws Exception {
        try (Connection connection = connectionFactory.createConnection()) {
            connection.start();
            Session session = connection.createSession(false, Session.AUTO_ACKNOWLEDGE);
            TemporaryQueue queue = session.createTemporaryQueue();
            assertThat(queue.getQueueName()).isNotBlank();
        }
    }
}
