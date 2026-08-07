package com.github.epex.mqproxy.config

import org.apache.activemq.ActiveMQConnectionFactory
import org.springframework.context.annotation.Bean
import org.springframework.context.annotation.Configuration

@Configuration
class JmsConfig(private val props: BrokerProperties) {

    @Bean
    fun connectionFactory(): ActiveMQConnectionFactory =
        ActiveMQConnectionFactory(props.username, props.password, props.url)
}
