package dev.cloudtui.mqproxy.config;

import jakarta.jms.ConnectionFactory;
import org.apache.activemq.ActiveMQConnectionFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * mq-proxy's own JMS connection to the broker (local embedded, or a real
 * Amazon MQ endpoint) — a separate credential pair from the HTTP Basic
 * Auth callers of this API use.
 */
@Configuration
public class BrokerConfig {

    @Bean
    public ConnectionFactory connectionFactory(
            @Value("${mqproxy.broker.url}") String brokerUrl,
            @Value("${mqproxy.broker.username:}") String username,
            @Value("${mqproxy.broker.password:}") String password) {
        ActiveMQConnectionFactory factory = new ActiveMQConnectionFactory(brokerUrl);
        if (!username.isBlank()) {
            factory.setUserName(username);
            factory.setPassword(password);
        }
        return factory;
    }
}
