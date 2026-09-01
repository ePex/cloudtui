package com.github.epex.mqproxy.config

import org.springframework.context.annotation.Bean
import org.springframework.context.annotation.Configuration
import org.springframework.security.config.annotation.web.builders.HttpSecurity
import org.springframework.security.config.annotation.web.configuration.EnableWebSecurity
import org.springframework.security.config.http.SessionCreationPolicy
import org.springframework.security.core.userdetails.User
import org.springframework.security.crypto.bcrypt.BCryptPasswordEncoder
import org.springframework.security.crypto.password.PasswordEncoder
import org.springframework.security.provisioning.InMemoryUserDetailsManager
import org.springframework.security.web.SecurityFilterChain
import org.springframework.web.cors.CorsConfiguration
import org.springframework.web.cors.CorsConfigurationSource
import org.springframework.web.cors.CorsUtils
import org.springframework.web.cors.UrlBasedCorsConfigurationSource

@Configuration
@EnableWebSecurity
class SecurityConfig(private val props: ProxyAuthProperties, private val corsProps: CorsProperties) {

    @Bean
    fun passwordEncoder(): PasswordEncoder = BCryptPasswordEncoder()

    @Bean
    fun userDetailsService(encoder: PasswordEncoder) = InMemoryUserDetailsManager(
        User.builder()
            .username(props.username)
            .password(encoder.encode(props.password))
            .roles("USER")
            .build()
    )

    // Lets the mq-proxy-web static console (a different origin than this
    // service) call the API. The "null" origin is what a browser sends for
    // a page opened directly via file:// (double-click, no server at all)
    // — always allowed, unconditionally, since there's no meaningful
    // narrower value to configure for it. Configured http(s):// origins
    // come from corsProps (proxy.cors.allowed-origins / CORS_ALLOWED_ORIGINS)
    // for served deployments. allowCredentials is left off: this API is
    // called with an explicit Authorization header the client sets itself,
    // not via cookies/credentialed fetch mode, so it isn't a "credentialed"
    // CORS request in the spec sense, and the origin allow-list here is a
    // deliberate access control rather than something CORS forces on us.
    @Bean
    fun corsConfigurationSource(): CorsConfigurationSource {
        val config = CorsConfiguration()
        config.allowedOrigins = corsProps.allowedOrigins + "null"
        config.allowedMethods = listOf("GET", "POST", "OPTIONS")
        config.allowedHeaders = listOf("Authorization", "Content-Type")
        val source = UrlBasedCorsConfigurationSource()
        source.registerCorsConfiguration("/api/**", config)
        return source
    }

    @Bean
    fun filterChain(http: HttpSecurity): SecurityFilterChain =
        http
            .cors { }
            .csrf { it.disable() }
            .sessionManagement { it.sessionCreationPolicy(SessionCreationPolicy.STATELESS) }
            .authorizeHttpRequests {
                // A CORS preflight (OPTIONS) request never carries the
                // Authorization header, so it must be let through before
                // the /api/** authenticated() rule below, or every
                // cross-origin call from mq-proxy-web would fail its
                // preflight with 401 before the real request is ever sent.
                it.requestMatchers(CorsUtils::isPreFlightRequest).permitAll()
                    .requestMatchers("/swagger-ui/**", "/swagger-ui.html", "/v3/api-docs/**", "/v3/api-docs.yaml").permitAll()
                    .requestMatchers("/api/**").authenticated()
                    .anyRequest().denyAll()
            }
            .httpBasic { }
            .build()
}
