package com.github.epex.mqproxy.api.model

data class ItemResponse<T>(
    val data: T?,
    val error: ApiError? = null,
)
