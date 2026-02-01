# Symbol Indexing and Boosting Implementation Plan

## 1. Overview
This document outlines the plan to enhance the `mcp-relic-server` search capabilities by indexing code symbols (functions, classes, structs, etc.). The goal is to "boost" search results where the query matches a *definition* of a symbol, prioritizing it over mere usages or comments.

## 2. Approach
We will use a **Regex-based Heuristic** approach.
- **Why:** It is lightweight, requires no external dependencies (like `ctags` or `tree-sitter` bindings), and is sufficiently accurate for boosting purposes.
- **How:** We will maintain a map of regex patterns per language extension. During indexing, we run these patterns against the file content to extract a list of symbol names.

## 3. Language Support (Initial)
| Language | Extensions | Elements Indexed |
| :--- | :--- | :--- |
| **Go** | `.go` | `func`, `type` (struct/interface), `const`, `var` |
| **Python** | `.py` | `def`, `class` |
| **JavaScript / TypeScript** | `.js`, `.ts`, `.jsx`, `.tsx` | `function`, `class`, `const/let/var` assignments |
| **Java** | `.java` | `class`, `interface`, `enum`, method definitions |
| **Rust** | `.rs` | `fn`, `struct`, `enum`, `trait`, `mod`, `type` |
| **C / C++** | `.c`, `.h`, `.cpp`, `.hpp`, `.cc` | `class`, `struct`, `#define`, functions |

## 4. Implementation Tasks

### Task 1: Symbol Extraction Logic
Create the core logic to extract symbols from text based on file extensions.

*   **Implementation**:
    *   Create `internal/gitrepos/symbols.go`.
    *   Define `LanguageRegex` struct and `languagePatterns` map.
    *   Implement `ExtractSymbols(ext, content string) []string`.
*   **Unit Tests (`internal/gitrepos/symbols_test.go`)**:
    *   Table-driven tests for every supported language.
    *   **Test Case**: Input a Go file snippet `func MyFunc() {}`. Assert output contains `["MyFunc"]`.
    *   **Test Case**: Input a Python file snippet `class MyClass:\n  def my_method(self):`. Assert output contains `["MyClass", "my_method"]`.
    *   **Test Case**: Input unsupported extension. Assert output is empty/nil.
    *   **Test Case**: Input malformed code or empty strings.

### Task 2: Data Model Update
Update the domain model to store these symbols.

*   **Implementation**:
    *   Modify `internal/domain/code.go`.
    *   Add `Symbols []string` field to `CodeDocument` struct.
    *   Add `const CodeFieldSymbols = "symbols"`.
*   **Tests**:
    *   No specific logic to test, but verifies the struct compiles.

### Task 3: Indexer Integration
Update the Bleve indexer to process and store the symbols.

*   **Implementation**:
    *   Modify `internal/gitrepos/indexer.go`.
    *   In `CreateIndexMapping`, add a mapping for the `symbols` field.
        *   **Type**: `TextFieldMapping`.
        *   **Analyzer**: `standard` (or `keyword` if we want exact matches only, but `standard` is usually better for search).
        *   **Store**: `false` (we don't need to retrieve the list of symbols, just search against it).
        *   **IncludeInAll**: `true`.
    *   In `FullIndex` and `IncrementalIndex`, call `ExtractSymbols` before creating the `CodeDocument`.
*   **Integration Tests**:
    *   Update `internal/gitrepos/indexer_test.go` (if exists) or create a new test.
    *   **Test**: Create an in-memory index. Index a document with known symbols. Verify that searching for a symbol name finds the document.

### Task 4: Search Query Boosting
Update the search handler to boost matches on the symbol field.

*   **Implementation**:
    *   Modify `internal/gitrepos/tools_search.go`.
    *   In `buildQuery`, add a specific query for the `symbols` field.
    *   **Boosting strategy**:
        *   Create a `DisjunctionQuery` (OR).
        *   Sub-query 1: Match `content` (boost: 1.0).
        *   Sub-query 2: Match `symbols` (boost: 2.0 or 5.0).
        *   This ensures that if a term appears in both, the score is higher. If it appears only in `symbols` (unlikely), it is also found.
*   **Integration Tests**:
    *   Create a test case in `tests/integration/gitrepos_test.go` (or similar).
    *   **Scenario**:
        1.  **Doc A**: Define `func ComputeX()`.
        2.  **Doc B**: Comment `// TODO: call ComputeX`.
        3.  **Action**: Search for "ComputeX".
        4.  **Assertion**: Doc A matches with a higher score than Doc B.

### Task 5: End-to-End Verification
Verify the feature works via the MCP interface.

*   **Manual/CLI Test**:
    *   Run the server.
    *   Index a sample repo.
    *   Use the `search` tool via the MCP inspector or a script.
    *   Verify results are returned and ranking makes sense.

## 5. Future Extensions
*   **Customization**: Allow users to add custom regexes via config.
*   **More Languages**: Add Ruby, PHP, Swift, etc.
*   **Better Parsers**: Evaluate integrating `tree-sitter` if regex proves too inaccurate (though unlikely for this use case).
