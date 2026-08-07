package com.github.epex.mqproxy.config

import io.swagger.v3.oas.models.OpenAPI
import io.swagger.v3.oas.models.info.Info
import io.swagger.v3.oas.models.security.SecurityRequirement
import io.swagger.v3.oas.models.security.SecurityScheme
import io.swagger.v3.oas.models.Components
import org.springframework.context.annotation.Bean
import org.springframework.context.annotation.Configuration

@Configuration
class OpenApiConfig {

    @Bean
    fun openApi(): OpenAPI = OpenAPI()
        .info(Info().title("mq-proxy API").version("1.0").description("ActiveMQ broker proxy"))
        .components(
            Components().addSecuritySchemes(
                "basicAuth",
                SecurityScheme()
                    .type(SecurityScheme.Type.HTTP)
                    .scheme("basic")
                    .description("Proxy credentials (proxy.auth.username / proxy.auth.password)")
            )
        )
        .addSecurityItem(SecurityRequirement().addList("basicAuth"))
}
