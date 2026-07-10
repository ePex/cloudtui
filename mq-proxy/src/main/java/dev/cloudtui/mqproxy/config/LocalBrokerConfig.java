package dev.cloudtui.mqproxy.config;

import org.apache.activemq.broker.BrokerPlugin;
import org.apache.activemq.broker.BrokerService;
import org.apache.activemq.plugin.StatisticsBrokerPlugin;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Profile;

/**
 * Starts an in-memory, non-persistent ActiveMQ broker in this same JVM —
 * the Docker-free local dev stack. Its connector matches
 * {@code mqproxy.broker.url}'s default, so {@link BrokerConfig}'s
 * connection factory needs no profile-specific override to reach it.
 */
@Configuration
@Profile("local")
public class LocalBrokerConfig {

    @Bean(initMethod = "start", destroyMethod = "stop")
    public BrokerService embeddedBroker() throws Exception {
        BrokerService broker = new BrokerService();
        broker.setBrokerName("cloudtui-local");
        broker.setPersistent(false);
        broker.setUseJmx(false);
        broker.addConnector("tcp://localhost:61616");
        // Amazon MQ enables this plugin on managed brokers by default, which
        // is what QueueService's per-queue stats query relies on — enabled
        // explicitly here since we own this broker's config.
        broker.setPlugins(new BrokerPlugin[] {new StatisticsBrokerPlugin()});
        return broker;
    }
}
