package com.github.epex.mqproxy

import org.springframework.boot.autoconfigure.SpringBootApplication
import org.springframework.boot.context.properties.ConfigurationPropertiesScan
import org.springframework.boot.runApplication

@SpringBootApplication
@ConfigurationPropertiesScan
class MqProxyApplication

fun main(args: Array<String>) {
    runApplication<MqProxyApplication>(*args)
}
