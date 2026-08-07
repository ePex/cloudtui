package com.github.epex.mqproxy.api

import com.github.epex.mqproxy.service.NotFoundException
import jakarta.jms.JMSException
import org.springframework.http.HttpStatus
import org.springframework.web.bind.annotation.ExceptionHandler
import org.springframework.web.bind.annotation.ResponseStatus
import org.springframework.web.bind.annotation.RestControllerAdvice

@RestControllerAdvice
class GlobalExceptionHandler {

    @ExceptionHandler(NotFoundException::class)
    @ResponseStatus(HttpStatus.NOT_FOUND)
    fun handleNotFound(ex: NotFoundException): Map<String, String> =
        mapOf("error" to (ex.message ?: "Not found"))

    @ExceptionHandler(JMSException::class)
    @ResponseStatus(HttpStatus.BAD_GATEWAY)
    fun handleJms(ex: JMSException): Map<String, String> =
        mapOf("error" to (ex.message ?: "Broker error"))

    @ExceptionHandler(Exception::class)
    @ResponseStatus(HttpStatus.INTERNAL_SERVER_ERROR)
    fun handleGeneric(ex: Exception): Map<String, String> =
        mapOf("error" to (ex.message ?: "Internal server error"))
}
