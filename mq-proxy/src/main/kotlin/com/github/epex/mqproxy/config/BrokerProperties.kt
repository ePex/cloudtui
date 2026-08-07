package com.github.epex.mqproxy.config

import org.springframework.boot.context.properties.ConfigurationProperties

@ConfigurationProperties("broker")
data class BrokerProperties(
    val url: String = "tcp://localhost:61616",
    val username: String = "admin",
    val password: String = "admin",
)
