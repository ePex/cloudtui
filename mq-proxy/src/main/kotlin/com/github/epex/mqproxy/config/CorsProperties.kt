package com.github.epex.mqproxy.config

import org.springframework.boot.context.properties.ConfigurationProperties

// Served http(s):// origins allowed to call this API cross-origin (e.g.
// wherever mq-proxy-web is hosted). The "null" origin (file:// pages) is
// always allowed separately, unconditionally — see SecurityConfig.
@ConfigurationProperties("proxy.cors")
data class CorsProperties(
    val allowedOrigins: List<String> = emptyList(),
)
