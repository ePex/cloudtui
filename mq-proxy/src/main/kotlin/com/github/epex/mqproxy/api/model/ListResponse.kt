package com.github.epex.mqproxy.api.model

data class ListResponse<T>(
    val data: List<T>,
    val errors: List<ApiError> = emptyList(),
)
