package com.github.epex.mqproxy.config

import org.springframework.boot.context.properties.ConfigurationProperties

@ConfigurationProperties("proxy.auth")
data class ProxyAuthProperties(
    val username: String = "cloudtui",
    val password: String = "changeme",
)
