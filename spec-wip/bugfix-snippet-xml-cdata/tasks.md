# Tasks: keep CDATA sections when formatting snippet XML

Each task is implemented and approved on its own, then pushed. Every new
test is mutation-checked: remove the fix and confirm the test fails, and
make sure the mutated code still compiles.

1. [x] **Keep CDATA in `formatXML`.**
   - Record each token's source span; a `CharData` token whose span is
     `<![CDATA[…]]>` keeps it in `xmlNode.raw`, and `writeXMLNode`
     writes it as is.
   - A node with `raw` set is never whitespace, in `hasMixedContent`,
     the whitespace-dropping in `writeXMLNode`, and `isXMLWhitespace`.
   - Out-of-range or backwards offsets leave the body unformatted.
   - New table cases in `format_test.go`, as listed in `plan.md`. All
     existing cases pass unchanged.
   - Mutation checks: no `raw` write, and whitespace-only CDATA treated
     as whitespace. The fallback is untestable: no valid input reaches
     it (see `plan.md`).
2. [ ] **Live verification.**
   - Use the `verify-live` skill with a temporary `HOME`, run from the
     scratch folder (not `tui/`), against the local broker. Check each
     step on screen and record the results here.
   1. In the Snippets view, create a snippet (`n`) with JMS Type
      `SoapCall` and a body with an XML envelope whose payload element
      holds a CDATA section with an embedded XML document. After saving,
      the file keeps the CDATA section byte for byte, and the envelope is
      indented.
   2. The preview shows the CDATA section as written.
   3. Open the snippet in the editor (`e`) and save again: the file is
      unchanged.
   4. Load it in the send dialog (**Load snippet…**) and send it to a
      throwaway queue. In the message detail view, the body shows the
      CDATA section.
3. [ ] **Merge-back** (needs your explicit go-ahead before it's
   committed).
   - `spec/22`: the File format note changes from "CDATA sections are
     written as escaped text" to "kept exactly as written".
   - Delete `spec-wip/bugfix-snippet-xml-cdata/` and mark the PR ready
     for review.
