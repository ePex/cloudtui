package com.github.epex.mqproxy.api

import jakarta.jms.JMSException
import org.springframework.http.HttpStatus
import org.springframework.validation.BindException
import org.springframework.web.bind.annotation.ExceptionHandler
import org.springframework.web.bind.annotation.ResponseStatus
import org.springframework.web.bind.annotation.RestControllerAdvice

@RestControllerAdvice
class GlobalExceptionHandler {

    @ExceptionHandler(JMSException::class)
    @ResponseStatus(HttpStatus.BAD_GATEWAY)
    fun handleJms(ex: JMSException): Map<String, String> =
        mapOf("error" to (ex.message ?: "Broker error"))

    // A missing/malformed required field on a @ModelAttribute-bound query
    // object (e.g. list-messages' ListMessagesQuery) fails during
    // constructor binding as a BindException, not the
    // MissingServletRequestParameterException plain @RequestParam gives —
    // without this handler it falls through to handleGeneric's 500 with a
    // raw Kotlin null-check message instead of a clean 400.
    @ExceptionHandler(BindException::class)
    @ResponseStatus(HttpStatus.BAD_REQUEST)
    fun handleBind(ex: BindException): Map<String, String> =
        mapOf("error" to (ex.bindingResult.allErrors.firstOrNull()?.defaultMessage ?: "Invalid request parameters"))

    @ExceptionHandler(Exception::class)
    @ResponseStatus(HttpStatus.INTERNAL_SERVER_ERROR)
    fun handleGeneric(ex: Exception): Map<String, String> =
        mapOf("error" to (ex.message ?: "Internal server error"))
}
