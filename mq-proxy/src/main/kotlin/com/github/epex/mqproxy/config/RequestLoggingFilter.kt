package com.github.epex.mqproxy.config

import jakarta.servlet.FilterChain
import jakarta.servlet.http.HttpServletRequest
import jakarta.servlet.http.HttpServletResponse
import org.slf4j.LoggerFactory
import org.springframework.stereotype.Component
import org.springframework.web.filter.OncePerRequestFilter

@Component
class RequestLoggingFilter : OncePerRequestFilter() {

    private val log = LoggerFactory.getLogger(RequestLoggingFilter::class.java)

    override fun doFilterInternal(
        request: HttpServletRequest,
        response: HttpServletResponse,
        chain: FilterChain,
    ) {
        val start = System.currentTimeMillis()
        try {
            chain.doFilter(request, response)
        } finally {
            val ms = System.currentTimeMillis() - start
            val query = request.queryString?.let { "?$it" } ?: ""
            log.info("{} {}{} -> {} ({}ms)", request.method, request.requestURI, query, response.status, ms)
        }
    }
}
