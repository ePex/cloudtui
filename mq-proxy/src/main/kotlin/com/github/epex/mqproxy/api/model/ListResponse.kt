package com.github.epex.mqproxy.api.model

// hasMore and truncated are only meaningful for list-messages (spec/11) -
// every other list-returning endpoint just leaves them at the default
// false. Shared here rather than forking a list-messages-specific
// response type for two booleans.
//
// hasMore: another page can be fetched (maxCount reached with more
// behind it) - what a "Load more" pages on. truncated: the broker cut the
// browse short at its browse limit, so the queue holds messages this
// browse couldn't see; paging can't reach them either.
data class ListResponse<T>(
    val data: List<T>,
    val errors: List<ApiError> = emptyList(),
    val hasMore: Boolean = false,
    val truncated: Boolean = false,
)
