package ui

// Filterable is implemented by views that support live filtering via the
// global '/' key. No current view implements it — this exists so future
// list/table views have a contract to implement rather than filtering
// being bolted on ad hoc.
type Filterable interface {
	Filter(query string)
}
