package com.github.epex.mqproxy.api.model

// hasMore is only meaningful for list-messages (spec/11's pagination) -
// every other list-returning endpoint just leaves it at the default
// false. Shared here rather than forking a list-messages-specific
// response type for one boolean.
data class ListResponse<T>(
    val data: List<T>,
    val errors: List<ApiError> = emptyList(),
    val hasMore: Boolean = false,
)
