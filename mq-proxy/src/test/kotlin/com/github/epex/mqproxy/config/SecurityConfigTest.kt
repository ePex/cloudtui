package com.github.epex.mqproxy.config

import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Test
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.boot.test.context.SpringBootTest
import org.springframework.boot.test.web.client.TestRestTemplate
import org.springframework.boot.test.web.server.LocalServerPort
import org.springframework.http.HttpHeaders
import org.springframework.http.HttpMethod
import org.springframework.http.HttpStatus
import org.springframework.http.RequestEntity
import org.springframework.test.context.TestPropertySource
import java.net.URI

// CORS preflight behavior (mq-proxy-web needs this to call the API
// cross-origin — see SecurityConfig.corsConfigurationSource). Uses a real
// embedded server (not @WebMvcTest) because Spring Security's CorsFilter
// short-circuits the filter chain before a mock-dispatcher-based test
// reliably exercises it — verified live that a @WebMvcTest slice here
// gave false 401s where the real running app correctly returned 200/403.
// No live broker needed: a rejected/short-circuited preflight never
// reaches QueueController, let alone BrokerService.
@SpringBootTest(webEnvironment = SpringBootTest.WebEnvironment.RANDOM_PORT)
@TestPropertySource(properties = ["proxy.cors.allowed-origins=http://allowed.example"])
class SecurityConfigTest {

    @LocalServerPort
    private var port: Int = 0

    @Autowired
    private lateinit var restTemplate: TestRestTemplate

    private fun preflight(origin: String) = restTemplate.exchange(
        RequestEntity<Void>(
            HttpHeaders().apply {
                set(HttpHeaders.ORIGIN, origin)
                set("Access-Control-Request-Method", "GET")
            },
            HttpMethod.OPTIONS,
            URI.create("http://localhost:$port/api/management/command/list-queues"),
        ),
        Void::class.java,
    )

    @Test
    fun `preflight from a configured allowed origin succeeds with matching CORS headers`() {
        val response = preflight("http://allowed.example")
        assertEquals(HttpStatus.OK, response.statusCode)
        assertEquals("http://allowed.example", response.headers.getFirst("Access-Control-Allow-Origin"))
    }

    @Test
    fun `preflight from the null origin (file colon-slash-slash pages) always succeeds`() {
        val response = preflight("null")
        assertEquals(HttpStatus.OK, response.statusCode)
        assertEquals("null", response.headers.getFirst("Access-Control-Allow-Origin"))
    }

    @Test
    fun `preflight from an origin not in the allow-list is rejected`() {
        val response = preflight("http://not-allowed.example")
        assertEquals(HttpStatus.FORBIDDEN, response.statusCode)
    }
}
