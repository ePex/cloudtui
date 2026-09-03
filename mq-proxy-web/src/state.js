// The page's single shared mutable state object. Every module that needs it
// imports this same object reference (ES module imports are live bindings,
// not copies), so a mutation from any module is visible everywhere else —
// the same behavior the old single-closure app.js had, just given a module
// boundary instead of a shared closure scope.
export const state = {
  conn: null,
  currentQueue: null,
  currentMessage: null,
  queues: [],
  queueSort: { column: 'name', direction: 'asc' },
  messages: [],
  messagesHasMore: false,
  selectedMessageIds: new Set(),
};
