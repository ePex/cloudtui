package dev.cloudtui.mqproxy.error;

import dev.cloudtui.mqproxy.generated.model.ErrorResponse;
import dev.cloudtui.mqproxy.queues.QueueNotFoundException;
import dev.cloudtui.mqproxy.queues.QueueOperationException;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;

/** Maps queue-operation failures onto the API's {@link ErrorResponse} contract. */
@RestControllerAdvice
public class ApiExceptionHandler {

    @ExceptionHandler(QueueNotFoundException.class)
    public ResponseEntity<ErrorResponse> handleNotFound(QueueNotFoundException e) {
        return ResponseEntity.status(HttpStatus.NOT_FOUND).body(new ErrorResponse(e.getMessage()));
    }

    @ExceptionHandler(QueueOperationException.class)
    public ResponseEntity<ErrorResponse> handleOperationError(QueueOperationException e) {
        return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR)
            .body(new ErrorResponse(e.getMessage()));
    }
}
