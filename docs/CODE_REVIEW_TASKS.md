# Code Review Tasks

Tasks identified from a full repository review of the RELIC MCP Server codebase, organized by severity and area.

---

## Verification Protocol (applies to every task)

After completing each task:

1. Run `make check` (format, lint, test) — must pass with zero failures
2. Review changed files in context of the entire library for consistency
3. Commit the task as a separate commit with a descriptive message

---

## Critical

### CR-1: Fix resource leak risk in `CreateMCPServer()` cleanup

**File:** `internal/app/runner.go:88-97`

**Problem:** If `svc.Initialize()` succeeds but `CreateServer()` panics or fails afterward, the cleanup function is never registered and the git repos service won't be closed.

**Fix:** Set the cleanup function *before* calling `Initialize()`, and clear it on initialization failure:

```go
cleanup = func() { svc.Close() }
if err := svc.Initialize(ctx); err != nil {
    cleanup()
    cleanup = nil
    // log error
} else {
    gitReposSvc = svc
}
```

**Test plan:**

*Happy path:*
- Update `TestCreateMCPServer_WithGitRepos` in `runner_test.go` to assert that `Close()` is called on the service when the returned cleanup function is invoked.

*Error paths:*
- Add `TestCreateMCPServer_CleanupCalledOnInitFailure`: inject a service whose `Initialize()` returns an error; verify `Close()` is called during error handling and cleanup is returned as nil.
- Add `TestCreateMCPServer_CleanupSetBeforeInitialize`: verify (via mock call ordering) that cleanup is assigned *before* `Initialize()` is called, so a panic in `Initialize()` doesn't leak resources.
- Add `TestCreateMCPServer_NewServiceFails`: inject settings that cause `gitrepos.NewService()` to fail (e.g., unwritable base directory); verify the error is propagated, cleanup is nil, and no resources are leaked.
- Add `TestCreateMCPServer_CloseErrorDuringInitFailure`: inject a service where both `Initialize()` and `Close()` return errors; verify `Close()` error is logged but the `Initialize()` error is the one that determines the outcome.

*Edge cases:*
- Add `TestCreateMCPServer_NilSettings`: pass nil git repos settings; verify graceful handling (no panic, no cleanup needed).
- Add `TestCreateMCPServer_DoubleCleanupCall`: invoke the returned cleanup function twice; verify `Close()` handles double-close safely (no panic).
- Verify existing `TestRunWithDeps_Cleanup` and `TestRunWithDeps_SSEWithNilCleanup` still pass.

**Effort:** Small

---

### CR-2: Fix index resource leak in `CreateAlias()`

**File:** `internal/gitrepos/indexer.go` — `CreateAlias()` function

**Problem:** Multiple Bleve indexes are opened. If alias creation or downstream code fails after some indexes are opened, those indexes aren't closed.

**Fix:** Collect opened indexes and add a deferred cleanup-on-error pattern:

```go
var opened []bleve.Index
defer func() {
    if err != nil {
        for _, idx := range opened {
            idx.Close()
        }
    }
}()
```

**Test plan:**

*Regression:*
- Existing `TestIndexer_CreateAlias`, `TestIndexer_CreateAlias_Empty`, and `TestIndexer_CreateAlias_NonExistent` must continue to pass.

*Error paths — partial open failure:*
- Add `TestIndexer_CreateAlias_PartialOpenFailure`: create 3 valid indexes, corrupt the 2nd (delete or truncate its directory); call `CreateAlias()`; verify it returns an error AND the 1st (successfully opened) index is closed (verify the index directory is not locked and can be re-opened).
- Add `TestIndexer_CreateAlias_LastRepoFails`: create N valid indexes, corrupt only the last; verify all N-1 successfully opened indexes are closed on error.
- Add `TestIndexer_CreateAlias_FirstRepoFails`: corrupt only the 1st repo; verify the error is returned immediately and no indexes are left open.

*Error paths — close failure during cleanup:*
- Add `TestIndexer_CreateAlias_CloseErrorDuringCleanup`: if feasible, simulate a scenario where closing an already-opened index returns an error during cleanup of a partial failure; verify the function still returns the original open error (not the close error) and attempts to close all opened indexes.

*Edge cases:*
- Add `TestIndexer_CreateAlias_SingleRepo`: verify alias works correctly with exactly 1 repo (not just N > 1).
- Add `TestIndexer_CreateAlias_PermissionDenied`: make an index directory unreadable; verify a clear error is returned and no file handles leak.
- Add `TestIndexer_CreateAlias_EmptyIndex`: create an alias over an index with zero documents; verify the alias is functional and search returns empty results.

**Effort:** Small

---

## High

### CR-3: Make git repos initialization failure more visible

**File:** `internal/app/runner.go:88-95`

**Problem:** The server starts successfully even when git repos fail to initialize. Users may not notice the `slog.Error` line. MCP tools then fail at invocation time with confusing errors.

**Fix:** Either:
- (A) Fail fast — return error if git repos can't initialize (breaking change).
- (B) Log a prominent startup banner/warning and expose a degraded health status.

**Test plan:**

*If option (A) — fail fast:*
- Update `TestCreateMCPServer_WithGitReposInitFailure` in `runner_test.go` to expect an error return instead of silent success.
- Update `TestRunWithDeps_ErrorCases` to cover the init-failure error path end-to-end (error propagated to caller).
- Add `TestCreateMCPServer_InitFailure_CleanupStillRuns`: verify `Close()` is called on the service even when returning the init error.

*If option (B) — degraded mode:*
- Add `TestCreateMCPServer_DegradedHealthStatus` in `server_test.go`: verify the health endpoint returns a degraded status (e.g., 503 or JSON `{"status":"degraded"}`) when git repos failed to initialize.
- Update `TestNewSSEServer_HealthEndpoint` to cover both healthy and degraded states.
- Add `TestCreateMCPServer_DegradedLogsWarning`: capture log output and verify a prominent warning (e.g., `slog.Warn` with "degraded") is emitted at startup.

*Error paths (both options):*
- Add `TestCreateMCPServer_InitFailure_LockError`: inject a service where lock acquisition fails; verify appropriate error/warning behavior.
- Add `TestCreateMCPServer_InitFailure_SyncError`: inject a service where SyncAll fails but lock succeeds; verify partial initialization is handled.
- Add `TestCreateMCPServer_InitFailure_NoURLs`: configure empty URLs list; verify behavior matches the chosen option.

*Edge cases:*
- Add `TestCreateMCPServer_InitFailure_ToolsNotRegistered`: when init fails, verify MCP tools are not registered on the server (nil `GitReposSvc` passed to `CreateServer`).
- Verify `TestCreateMCPServer_WithGitRepos` (happy path) and `TestRunWithDeps_Cleanup` still pass.

**Effort:** Small–Medium

---

### CR-4: Use caller context in `Initialize()` call

**File:** `internal/app/runner.go:88`

**Problem:** `svc.Initialize(context.Background())` ignores the `ctx` passed to `CreateMCPServer()`. Initialization can't be cancelled or timed out by the caller.

**Fix:** Replace `context.Background()` with `ctx`.

**Test plan:**

*Happy path:*
- Existing `TestCreateMCPServer_WithGitRepos` passes a valid context; must continue to pass.

*Error paths:*
- Add `TestCreateMCPServer_RespectsContextCancellation`: pass a pre-cancelled context; inject a mock service whose `Initialize()` checks `ctx.Err()`; verify `Initialize()` receives the cancelled context and returns `context.Canceled`.
- Add `TestCreateMCPServer_ContextDeadlineExceeded`: pass a context with an already-expired deadline; verify `Initialize()` receives `context.DeadlineExceeded`.
- Add `TestCreateMCPServer_ContextCancelledDuringInit`: pass a context that is cancelled *during* `Initialize()` (use a goroutine to cancel after a short delay); verify `Initialize()` returns a context error and cleanup runs.

*Edge cases:*
- Add `TestCreateMCPServer_ContextWithValues`: pass a context carrying values; verify the context (including values) is forwarded to `Initialize()` unchanged.
- Verify `TestRunWithDeps_StdioWithDefaultTransport` still works (uses a cancelled context for a different purpose).

**Effort:** Small

---

### CR-5: Implement graceful HTTP server shutdown

**File:** `internal/app/server.go`

**Problem:** `StartSSEServer()` calls `srv.ListenAndServe()` which blocks forever. There's no `Shutdown(ctx)` path when the parent context is cancelled.

**Fix:** Accept a context parameter and run shutdown in a goroutine:

```go
func StartSSEServer(ctx context.Context, s *mcp.Server, settings *config.Settings) error {
    srv, err := NewSSEServer(s, settings)
    // ...
    go func() {
        <-ctx.Done()
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        srv.Shutdown(shutdownCtx)
    }()
    return srv.ListenAndServe()
}
```

**Test plan:**

*Happy path:*
- Add `TestStartSSEServer_GracefulShutdown` in `server_test.go`: start the server with a cancellable context on a free port (`testkit.MustGetFreePort`); verify it's serving (HTTP GET to `/health` returns 200); cancel the context; verify `StartSSEServer()` returns `http.ErrServerClosed` and the port is released.

*Error paths:*
- Add `TestStartSSEServer_PortAlreadyInUse`: bind a listener to a port first, then call `StartSSEServer` with the same port; verify it returns a bind error immediately.
- Add `TestStartSSEServer_InvalidAddress`: pass settings with an invalid host (e.g., `"999.999.999.999"`); verify a clear error is returned.
- Add `TestStartSSEServer_NewSSEServerFails`: pass invalid auth settings so `NewSSEServer()` fails; verify the error is propagated without starting the listener.
- Add `TestStartSSEServer_ShutdownWithActiveConnections`: start the server, open a long-lived SSE connection, cancel the context; verify the server shuts down within the timeout (connection drained or force-closed).

*Edge cases:*
- Add `TestStartSSEServer_ShutdownTimeout`: verify the server shuts down within the configured timeout (e.g., < 6 seconds) after context cancellation — use a deadline assertion.
- Add `TestStartSSEServer_ImmediateContextCancel`: pass a pre-cancelled context; verify `ListenAndServe` still starts but `Shutdown` is triggered immediately, returning `http.ErrServerClosed`.
- Add `TestStartSSEServer_HealthDuringShutdown`: cancel the context, then immediately hit `/health`; verify it either returns 200 (still draining) or connection refused (already stopped) — no panic or hang.

*Regression:*
- Update all callers of `StartSSEServer` in `runner.go` and tests to pass a context. Verify `TestRunWithDeps_ErrorCases` and all SSE-related tests still pass.
- Existing `TestNewSSEServer_HealthEndpoint` and auth tests are unaffected (they test `NewSSEServer`, not `StartSSEServer`).

**Effort:** Medium

---

## Medium

### CR-6: Print error message in `main()` before exit

**File:** `cmd/relic-mcp/main.go:26-28`

**Problem:** When `Execute()` returns an error, the process exits with code 1 but prints nothing to stderr.

**Fix:**

```go
if err := Execute(...); err != nil {
    fmt.Fprintf(os.Stderr, "Error: %v\n", err)
    exit(1)
}
```

**Test plan:**

*Happy path:*
- Existing `TestRunMain_Success` and `TestExecute_Help`/`TestExecute_Version` must continue to pass — no stderr output on success.

*Error paths:*
- Update `TestRunMain_Failure` in `main_test.go` to capture stderr output and assert it contains `"Error: "` prefix with the underlying error message. The test already verifies `exit(1)`.
- Add `TestRunMain_InvalidFlag_PrintsError`: invoke with `--invalid-flag`; capture stderr; assert output includes the flag name and a meaningful description.
- Add `TestRunMain_InvalidTransport_PrintsError`: invoke with `--transport=invalid`; capture stderr; assert output includes `"transport"` and the invalid value.

*Edge cases:*
- Add `TestRunMain_EmptyArgs`: invoke with empty args slice; verify normal execution (no error, no stderr output).
- Add `TestRunMain_ErrorMessageFormat`: verify the stderr message format is `"Error: <message>\n"` — exactly one newline, no extra whitespace, no stack trace.
- Existing `TestExecute_InvalidFlag` and `TestExecute_InvalidTransport` must still pass (they test the `Execute()` return value, not stderr).

**Effort:** Small

---

### CR-7: Return error from `CreateServer()`

**File:** `internal/mcp/server.go`

**Problem:** If MCP SDK server creation or tool registration fails, the function returns a potentially broken server with no error signal.

**Fix:** Change signature to `CreateServer(cfg ServerConfig) (*mcp.Server, error)` and propagate registration errors. Update all callers.

**Test plan:**

*Happy path (update existing tests for new signature):*
- Update `TestCreateServer` — assert `err == nil` and server is non-nil.
- Update `TestCreateServer_EmptyConfig` — assert `err == nil` with empty config.
- Update `TestCreateServer_WithVersion` — assert `err == nil` with version set.
- Update `TestCreateServer_WithoutGitReposService` — nil `GitReposSvc`; assert `err == nil` and no tools registered.
- Update `TestCreateServer_WithGitReposService` — assert `err == nil` and tools are registered.

*Error paths:*
- Add `TestCreateServer_RegistrationError`: if SDK's `AddTool` can return errors, mock a failing registration and assert the error is propagated. If the SDK doesn't return errors, document this as a comment in the test file and skip.
- Add `TestCreateServer_NilServerFromSDK`: if `mcp.NewServer()` can return nil (verify SDK behavior), test that `CreateServer` returns an error rather than a nil-pointer panic.

*Edge cases:*
- Add `TestCreateServer_TypedNilGitReposSvc`: pass a typed nil `(*ConcreteService)(nil)` that satisfies `GitReposToolService`; verify tools are NOT registered (the nil interface check must catch typed nils, or document that it doesn't).
- Add `TestCreateServer_EmptyNameAndVersion`: pass `Name: ""` and `Version: ""`; verify server is created without error (no validation panic).

*Caller updates:*
- Update `CreateMCPServer` in `runner.go` to handle the new error. Update `TestCreateMCPServer_WithGitRepos` and `TestRunWithDeps_ErrorCases` in `runner_test.go`.
- Add `TestRunWithDeps_CreateServerFails` in `runner_test.go`: inject a config that makes `CreateServer` return an error; verify `RunWithDeps` propagates it.
- Verify integration test `TestMCPServer_ToolsRegistered` in `tests/integration/gitrepos_test.go` still passes.

**Effort:** Medium

---

### CR-8: Replace wildcard query for repository filtering

**File:** `internal/gitrepos/tools_search.go:117`

**Problem:** `bleve.NewWildcardQuery("*" + args.Repository + "*")` can be expensive at scale with leading wildcards.

**Fix:** Consider using a prefix query, a regexp query, or indexing the repository field with an ngram analyzer for substring matching.

**Test plan:**

*Regression (identical results required):*
- `TestSearchHandler_SearchWithRepositoryFilter`, `TestSearchHandler_SearchWithBothFilters`, `TestSearchHandler_SubstringRepoFilter` in `tools_search_test.go` must produce identical results.
- Integration test `TestSearchTool_SearchWithRepositoryFilter` in `tests/integration/gitrepos_test.go` must pass.

*Error paths:*
- Add `TestSearchHandler_RepoFilter_SpecialWildcardChars`: set repository to `"my*repo"`, `"my?repo"`, `"my[test]repo"`; verify no query parse error or panic — either returns results correctly or returns zero results gracefully.
- Add `TestSearchHandler_RepoFilter_VeryLongName`: set repository to a 10,000-character string; verify no panic or excessive latency (should return zero results or an error, not hang).

*Edge cases:*
- Add `TestSearchHandler_RepoFilter_CaseInsensitive`: search for `"MyRepo"` when indexed as `"myrepo"`; verify behavior and document whether case-insensitive matching is supported.
- Add `TestSearchHandler_RepoFilter_ExactMatch`: repository name that matches exactly one indexed repo; verify it's returned.
- Add `TestSearchHandler_RepoFilter_NoMatch`: repository name that matches nothing; verify empty results (not an error).
- Add `TestSearchHandler_RepoFilter_PartialSlash`: filter with `"org/repo"` when full name is `"github.com/org/repo"`; verify substring matching works.

*Index mapping (if changed):*
- Add `TestCreateIndexMapping_RepositoryField` in `indexer_test.go` to verify the repository field uses the expected analyzer (keyword, ngram, or whatever the fix uses).

**Effort:** Medium

---

### CR-9: Harden `GetRepoState()` return semantics

**File:** `internal/gitrepos/manifest.go:104-112`

**Problem:** Returns a pointer to a `RepoState` value that doesn't write back to the map. Callers must remember to call `SetRepoState()` after mutation. Currently correct but fragile.

**Fix:** Return a copy (value, not pointer), or add a doc comment making the contract explicit. Alternatively, provide an `UpdateRepoState(id, func(*RepoState))` method that handles the read-modify-write atomically.

**Test plan:**

*If adding `UpdateRepoState(id, func(*RepoState))`:*
- Add `TestManifest_UpdateRepoState_Existing` in `manifest_test.go`: update an existing repo's URL and commit; verify changes are persisted in the manifest.
- Add `TestManifest_UpdateRepoState_NewRepo`: update a non-existent repo ID; verify it's created with the callback's mutations applied.
- Add `TestManifest_UpdateRepoState_ConcurrentUpdates`: launch 10 goroutines updating the same repo ID concurrently; verify no data race (run with `-race`) and final state is consistent.
- Add `TestManifest_UpdateRepoState_EmptyID`: pass empty string as ID; verify behavior (error or creates entry for `""`).
- Add `TestManifest_UpdateRepoState_NilCallback`: pass nil callback; verify no panic (no-op or error).

*If changing to return-by-value:*
- Update `TestManifest_GetRepoState_Existing`: get state, mutate the returned copy, get state again; assert the manifest's internal state is unchanged (key correctness assertion).
- Add `TestManifest_GetRepoState_MutationDoesNotPersist`: get state, modify `URL` field, call `GetRepoState` again; verify `URL` is unchanged.
- Add `TestManifest_GetRepoState_ConcurrentReadWrite`: one goroutine reads via `GetRepoState`, another writes via `SetRepoState`; verify no race and each sees a consistent snapshot.

*Error paths (both approaches):*
- Add `TestManifest_GetRepoState_NilReposMap`: create a manifest with `Repos` set to nil (simulate corrupted deserialization); verify no panic — either returns zero-value state or initializes the map.

*Regression:*
- Existing `TestManifest_SetRepoState`, `TestManifest_GetRepoState_New`, `TestManifest_HasRepo`, `TestManifest_RemoveRepo` must pass.
- Update all callers in `service.go` (`SyncAll`, `syncRepo`, `openIndexes`) to use the new API. Verify `TestService_Initialize_LeaderSyncSuccess` and related service tests pass.
- If `ManifestOperations` interface in `interfaces.go` changes, update mock in `mocks_test.go`.

**Effort:** Small–Medium

---

### CR-10: Use separate lock-wait timeout

**File:** `internal/gitrepos/service.go:133`

**Problem:** Follower instances reuse `SyncTimeout` as the lock wait duration. A separate, shorter timeout would better express intent.

**Fix:** Add a `LockWaitTimeout` field to config or derive it (e.g., `SyncTimeout / 2`).

**Test plan:**

*If adding a config field:*
- Add `TestLoadSettings_GitReposLockWaitTimeout` in `settings_test.go`: set env var, verify it's loaded into the new field.
- Add `TestLoadSettingsWithFlags_LockWaitTimeoutFlag`: set via CLI flag, verify flag overrides env var.
- Add `TestValidateSettings_GitReposInvalidLockWaitTimeout`: set to 0 and negative; expect validation error "must be positive".
- Add `TestValidateSettings_GitReposLockWaitTimeoutValid`: set to a positive value; expect no error.

*If deriving from `SyncTimeout`:*
- Add `TestService_LockWaitTimeoutDerivedFromSyncTimeout` in `service_test.go`: set `SyncTimeout` to 60s; verify the derived lock wait timeout is the expected fraction (e.g., 30s).
- Add `TestService_LockWaitTimeout_SmallSyncTimeout`: set `SyncTimeout` to 1s; verify derived timeout is reasonable (not zero or negative).

*Error paths:*
- Update `TestService_Initialize_FollowerTimeout` to use the new timeout value and verify the follower times out at the expected duration (not `SyncTimeout`).
- Add `TestService_Initialize_FollowerTimeout_ShortLockWait`: set lock wait to 100ms with a leader holding the lock; verify the follower times out quickly and proceeds to open indexes.

*Edge cases:*
- Add `TestService_Initialize_LockWaitEqualsZero`: if the derived or configured value is accidentally 0, verify `Lock()` either fails immediately or is handled gracefully.
- Existing `TestService_Initialize_FollowerSuccess` and `TestService_Initialize_LeaderSyncSuccess` must still pass.
- If config struct changes, update `testkit.NewTestFlags` in `tests/integration/testkit/testkit.go` if the flag is exposed.

**Effort:** Small

---

### CR-11: Stop silently ignoring Viper bind errors

**File:** `internal/config/settings.go:83-112`

**Problem:** All `v.BindEnv()` and `v.BindPFlag()` return values are discarded with `_`.

**Fix:** Collect errors or fail fast:

```go
if err := v.BindEnv("auth.type", "RELIC_MCP_AUTH_TYPE"); err != nil {
    return nil, fmt.Errorf("failed to bind env var: %w", err)
}
```

**Test plan:**

*Regression:*
- All existing `TestLoadSettings_*` and `TestLoadSettingsWithFlags_*` tests must continue to pass (bind calls succeed in normal operation).

*Error paths:*
- Add `TestLoadSettingsWithFlags_MissingFlag`: pass a `pflag.FlagSet` missing a required flag (e.g., no `"transport"` flag defined); verify `LoadSettingsWithFlags` returns a descriptive error mentioning the missing flag name (if `BindPFlag` with nil returns an error), or document that nil flags are silently skipped.
- Add `TestLoadSettingsWithFlags_EmptyFlagSet`: pass a completely empty `pflag.FlagSet` (no flags registered); verify either all bindings fail with errors or env vars/defaults still work correctly.

*Edge cases:*
- Add `TestLoadSettingsWithFlags_TypeMismatch`: register a flag as string type but set it to a non-numeric value where config expects int (e.g., `flags.String("port", "abc", "")`); verify the error surfaces during `Unmarshal`, not silently.
- Add `TestLoadSettingsWithFlags_DuplicateBindEnv`: verify that calling `LoadSettingsWithFlags` twice doesn't cause double-bind errors or unexpected behavior.
- Add `TestLoadSettings_BindEnvWithMissingEnvVar`: ensure that when an env var is not set, `BindEnv` still succeeds and the default value is used (this is the expected Viper behavior; test it to catch regressions).

**Effort:** Small

---

### CR-12: Trim auth credentials before validation

**File:** `internal/config/settings.go:223`

**Problem:** Whitespace-only passwords pass the `!= ""` check.

**Fix:** `strings.TrimSpace()` on username, password, and API keys before validation.

**Test plan:**

*Error paths — whitespace-only credentials:*
- Add `TestValidateSettings_BasicAuth_WhitespaceOnlyPassword`: password `"   "` → validation error.
- Add `TestValidateSettings_BasicAuth_WhitespaceOnlyUsername`: username `" \t "` → validation error.
- Add `TestValidateSettings_BasicAuth_TabOnlyPassword`: password `"\t"` → validation error.
- Add `TestValidateSettings_BasicAuth_NewlineOnlyPassword`: password `"\n"` → validation error.
- Add `TestValidateSettings_APIKey_WhitespaceOnlyKey`: API keys `[" "]` → validation error (no valid keys after trim).
- Add `TestValidateSettings_APIKey_MixedValidAndWhitespace`: API keys `["valid-key", "  ", "another-key"]` → validation passes, whitespace-only key is filtered out.

*Edge cases — special characters:*
- Add `TestValidateSettings_BasicAuth_PasswordWithNullByte`: password `"pass\x00word"` → verify behavior (either accept or reject, but no panic).
- Add `TestValidateSettings_BasicAuth_UnicodeWhitespace`: password `"\u00a0"` (non-breaking space) → document whether `TrimSpace` catches this (it doesn't — only ASCII whitespace). If not caught, add explicit handling or document as known limitation.
- Add `TestValidateSettings_BasicAuth_TrimmedCredentials`: username `" admin "` and password `" pass "` → trimmed, validation passes.
- Add `TestValidateSettings_APIKey_LeadingTrailingSpaces`: API keys `[" key1 ", "key2 "]` → verify keys are trimmed before use.

*Edge cases — auth type with whitespace:*
- Add `TestValidateSettings_AuthType_LeadingTrailingSpaces`: auth type `" basic "` → document whether it's trimmed to `"basic"` or rejected as unknown. If not trimmed, add trimming.

*Regression:*
- Existing `TestValidateSettings_ValidBasic`, `TestValidateSettings_ValidAPIKey`, `TestValidateSettings_BasicAuthMissingPassword`, `TestValidateSettings_BasicAuthMissingUsername` must still pass.
- If trimming is applied at load time (not just validation), update `TestLoadSettings_EnvVars` to verify trimmed values.

**Effort:** Small

---

### CR-13: Add missing `CodeFieldSymbols` test

**File:** `internal/domain/code_test.go:94-113`

**Problem:** The `TestCodeFieldConstants` table tests all field constants except `CodeFieldSymbols`.

**Fix:** Add the missing entry:

```go
{"CodeFieldSymbols", CodeFieldSymbols, "symbols"},
```

**Test plan:**

*Missing coverage:*
- Add `{"CodeFieldSymbols", CodeFieldSymbols, "symbols"}` to the table in `TestCodeFieldConstants`.
- Update `TestCodeDocument_JSONFieldNames`: add `Symbols: []string{"func1", "func2"}` to the test document; add `CodeFieldSymbols` to `expectedFields` map; verify the JSON key is `"symbols"` and the value is the expected array.

*Edge cases:*
- Add `TestCodeDocument_JSONMarshal_WithSymbols`: marshal a `CodeDocument` with a non-empty `Symbols` field; unmarshal and verify the symbols are preserved.
- Add `TestCodeDocument_JSONMarshal_NilSymbols`: marshal a `CodeDocument` with `Symbols: nil`; unmarshal and verify `Symbols` is either nil or empty slice (document which).
- Add `TestCodeDocument_JSONMarshal_EmptySymbols`: marshal with `Symbols: []string{}`; unmarshal and verify behavior differs from nil (or not — document).

*Regression:*
- All existing domain tests (`TestCodeDocument_JSONMarshal`, `TestCodeDocument_JSONUnmarshal`, `TestCodeDocument_EmptyFields`) must continue to pass.

**Effort:** Small

---

## Low

### CR-14: Add retry logic for transient git failures

**Files:** `internal/gitrepos/git.go` — `Clone()`, `Fetch()`

**Problem:** Network errors cause immediate failure with no retry.

**Fix:** Add configurable retry with exponential backoff (e.g., 3 attempts, 1s/2s/4s delays).

**Test plan:**

*Happy path:*
- Add `TestGitClient_Clone_RetriesOnTransientError` in `git_test.go`: `MockExecutor` fails first 2 calls, succeeds on 3rd; verify clone succeeds and all 3 attempts recorded.
- Add `TestGitClient_Fetch_RetriesOnTransientError`: same pattern for fetch.
- Add `TestGitClient_Clone_SucceedsOnFirstTry`: verify no retry delay when first attempt succeeds (fast path).

*Error paths:*
- Add `TestGitClient_Clone_ExhaustsRetries`: fail all N attempts; verify the *last* error is returned, the correct number of attempts were made, and no partial state is left.
- Add `TestGitClient_Fetch_ExhaustsRetries`: same for fetch.
- Add `TestGitClient_Clone_NonRetryableError`: return a non-transient error (e.g., auth failure, invalid URL); verify no retry is attempted — fails immediately on first try.
- Add `TestGitClient_Clone_DiskFullError`: simulate disk full; verify no retry (not transient).

*Edge cases — context:*
- Add `TestGitClient_Clone_RetryRespectsContext`: cancel context during retry backoff; verify returns `context.Canceled` promptly without waiting for remaining retries.
- Add `TestGitClient_Clone_RetryRespectsDeadline`: set a short deadline; verify returns `context.DeadlineExceeded` even if retries remain.
- Add `TestGitClient_Fetch_RetryContextCancelled`: same pattern for fetch.

*Edge cases — backoff:*
- Add `TestGitClient_Clone_BackoffTiming`: verify retry delays follow exponential backoff pattern (1s, 2s, 4s or similar). Use mock clock or measure elapsed time with tolerance.
- Add `TestGitClient_Clone_ZeroRetries`: if retry count is configurable and set to 0, verify no retries — single attempt only.

*Regression:*
- Existing `TestGitClient_Clone`, `TestGitClient_Clone_Error`, `TestGitClient_Fetch`, `TestGitClient_Fetch_Error` must pass (single-attempt behavior preserved).
- If retry count is configurable, add `TestLoadSettings_GitReposRetryCount` and `TestValidateSettings_GitReposInvalidRetryCount` in `settings_test.go`.

**Effort:** Medium

---

### CR-15: Implement Bleve index compaction / size monitoring

**File:** `internal/gitrepos/indexer.go`

**Problem:** Index files grow indefinitely. Stale repos are cleaned up, but no compaction runs.

**Fix:** Add periodic index optimization or compaction step during sync cycles. Log index sizes for observability.

**Test plan:**

*Happy path:*
- Add `TestIndexer_GetIndexSize` in `indexer_test.go`: create an index, add 10 documents; verify `GetIndexSize()` returns a positive value.
- Add `TestIndexer_GetIndexSize_AfterDelete`: add documents, delete some, verify size is still reported (may not decrease without compaction).

*Compaction (if Bleve supports it):*
- Add `TestIndexer_CompactIndex`: add 100 documents, delete 90, run compaction; verify index is still searchable and size decreased or stayed the same.
- Add `TestIndexer_CompactIndex_EmptyIndex`: compact an index with zero documents; verify no error and index remains functional.
- Add `TestIndexer_CompactIndex_DuringSearch`: run compaction while a search is in progress; verify neither operation fails (no locking issues).

*Error paths:*
- Add `TestIndexer_GetIndexSize_NonExistentIndex`: call on a non-existent index path; verify error (not panic).
- Add `TestIndexer_GetIndexSize_ClosedIndex`: close an index then call GetIndexSize; verify error.
- Add `TestIndexer_CompactIndex_PermissionDenied`: make index directory read-only; verify error during compaction.

*Observability:*
- Add `TestService_SyncLogsIndexSize` in `service_test.go`: capture log output; verify index sizes are logged after sync completes.
- Add `TestService_SyncLogsIndexSize_ZeroDocuments`: verify logging works correctly for empty indexes.

*Regression:*
- Existing `TestIndexer_FullIndex`, `TestIndexer_IncrementalIndex_*`, `TestIndexer_DeleteIndex`, and `TestIndexer_GetDocumentCount` must pass.
- If compaction runs during sync, verify `TestService_Initialize_LeaderSyncSuccess` timing is reasonable.

**Effort:** Medium

---

### CR-16: Improve Java symbol regex precision

**File:** `internal/gitrepos/symbols.go:39`

**Problem:** The method pattern may over-match on multi-line signatures or complex generics.

**Fix:** Tighten the regex or add post-match validation. Add test cases for complex generic signatures.

**Test plan:**

Add test cases to the `TestExtractSymbols` table in `symbols_test.go` for `.java` extension:

*Generic methods:*
- `public <T extends Comparable<T>> List<T> sort(List<T> items) {` → extract `sort`
- `private Map<String, List<Integer>> getMapping() {` → extract `getMapping`
- `protected static <K, V> Map<K, V> of(K key, V value) {` → extract `of`
- `public <T> Optional<T> find(Predicate<T> p) {` → extract `find`

*Return type variations:*
- `public int[] toArray() {` → extract `toArray`
- `public int[][] matrix() {` → extract `matrix`
- `public void process(String... args) {` → extract `process` (varargs)

*Modifiers and annotations:*
- `@Override public String toString() {` → extract `toString` (annotation-prefixed)
- `@Deprecated @SuppressWarnings("unchecked") public void old() {` → extract `old`
- `public synchronized void sync() {` → extract `sync`
- `public static final int VALUE = 42;` → verify this is NOT captured as a method

*Multi-line signatures:*
- `public void\n    processData(\n    String input) {` → document expected behavior (match or skip)

*Constructs that should NOT match as methods:*
- `if (condition) {` → must NOT be extracted
- `while (true) {` → must NOT be extracted
- `for (int i = 0; i < n; i++) {` → must NOT be extracted
- `switch (value) {` → must NOT be extracted

*Edge cases:*
- Interface default methods: `default void method() {` → document behavior
- Package-private method (no modifier): `void method() {` → document behavior
- Constructor: `public MyClass() {` → document behavior (no return type)

*Regression:*
- All existing Java test cases must pass with identical results.
- No regressions in other languages (table-driven structure ensures this).

**Effort:** Small

---

### CR-17: Support multi-line C function signatures

**File:** `internal/gitrepos/symbols.go:73`

**Problem:** Single-line pattern misses multi-line function definitions common in C codebases.

**Fix:** Use a multiline regex or a two-pass approach (find open paren, scan back for return type and name).

**Test plan:**

Add test cases to the `TestExtractSymbols` table in `symbols_test.go` for `.c` extension:

*Multi-line function signatures:*
- `int\nprocess_data(\n    const char *input,\n    int len)\n{` → extract `process_data`
- `char *\nget_name(void)\n{` → extract `get_name`
- `void handle_request(\n    int fd,\n    struct request *req,\n    struct response *res)\n{` → extract `handle_request`
- `static int\nhelper(\n    void)\n{` → extract `helper`

*Storage class and qualifier variations:*
- `static void internal_func() {` → extract `internal_func`
- `extern int exported_func(void) {` → extract `exported_func`
- `inline void fast_func() {` → extract `fast_func`
- `const char *get_string(void) {` → extract `get_string`

*Pointer return types:*
- `int *alloc_int(void) {` → extract `alloc_int`
- `struct node **get_list(void) {` → extract `get_list`
- `void *malloc_wrapper(size_t n) {` → extract `malloc_wrapper`

*Constructs that must NOT match:*
- `if (condition) {` → must NOT be extracted
- `while (true) {` → must NOT be extracted
- `for (int i = 0; i < n; i++) {` → must NOT be extracted
- `switch (value) {` → must NOT be extracted
- `} else {` → must NOT be extracted

*Forward declarations (should NOT match — no body):*
- `int func(void);` → must NOT be extracted (no `{`)
- `extern void callback(int n);` → must NOT be extracted

*Edge cases:*
- Function with `__attribute__`: `void func(void) __attribute__((unused)) {` → document behavior
- Function-like macro: `#define FUNC(x) ((x) + 1)` → captured by macro pattern, NOT by function pattern
- Typedef'd return types: `status_t process(void) {` → extract `process`
- Empty parameter list: `void noop() {` → extract `noop`

*Cross-extension:*
- Add corresponding test cases for `.h`, `.cpp`, `.cxx`, `.hpp` if the same patterns apply.

*Regression:*
- All existing C, C++, and header file test cases must continue to pass.

**Effort:** Medium

---

### CR-18: Add `-race` flag to CI test runs

**File:** `.github/workflows/ci.yml`

**Problem:** No race condition detection in CI. Concurrent sync and search paths could have hidden data races.

**Fix:** Add `-race` flag to `go test` in the CI workflow.

**Test plan:**

*Pre-merge validation:*
- Run `go test -race ./...` locally and fix any detected races before merging. Document each race found and its fix as a sub-task.

*Known risk areas to watch:*
- `internal/gitrepos/service.go`: concurrent `SyncAll` with goroutines writing to shared manifest state.
- `internal/gitrepos/manifest.go`: `RWMutex` usage in `GetRepoState`/`SetRepoState` — pointer returned after unlock creates a race window.
- `internal/gitrepos/filelock.go`: `TestFileLock_ConcurrentGoroutines` (10 goroutines, 5 ops each).
- `internal/gitrepos/service.go`: `GetIndexAlias()` reads `s.ready` and `s.alias` — verify mutex protects both.
- `internal/gitrepos/service.go`: `Close()` sets `s.ready = false` and `s.alias = nil` — verify no concurrent reader sees partial state.

*CI validation:*
- Verify CI pipeline passes with `-race` enabled. Ensure the CI job timeout accommodates the ~2-3x slowdown from the race detector.
- Verify memory usage stays within CI runner limits (race detector increases memory ~5-10x per goroutine).

*Edge cases:*
- If `-race` causes flaky tests (timing-sensitive tests), fix the flakiness or add appropriate synchronization rather than disabling `-race`.

**Effort:** Small

---

### CR-19: Add concurrent search and load tests

**Files:** `tests/integration/gitrepos_test.go`

**Problem:** No tests for concurrent search operations, large result sets, or corrupted index recovery.

**Fix:** Add integration tests that exercise:
- Concurrent search requests during sync
- Large result set pagination
- Recovery from corrupted index files

**Test plan:**

*Concurrency tests (run with `-race`):*
- Add `TestSearchTool_ConcurrentSearches`: launch 10 goroutines, each performing 5 searches concurrently; verify all return valid results, no panics, no data races.
- Add `TestSearchTool_ConcurrentSearchDuringSync`: start a sync operation and simultaneously issue search requests from multiple goroutines; verify searches either succeed with stale data or return "not ready" — never panic or return corrupted data.
- Add `TestSearchTool_ConcurrentSearchAndRead`: mix concurrent search and read-file operations; verify no interference between the two.

*Large result sets:*
- Add `TestSearchTool_LargeResultSet`: index 50+ files with overlapping content; search with a broad query; verify pagination footer ("more results available") appears when results exceed `MaxResults` and returned count equals `MaxResults`.
- Add `TestSearchTool_LargeResultSet_MaxResultsOne`: set `MaxResults=1`; verify exactly 1 result returned with pagination footer.

*Corrupted index recovery:*
- Add `TestIndex_CorruptedIndexRecovery`: create a valid index, corrupt it (truncate a file, write garbage to a segment); attempt to open — verify a clear error. Then re-index from scratch; verify the index is functional again.
- Add `TestIndex_CorruptedIndex_OpenForRead`: corrupt an index directory; call `OpenForRead()`; verify error (not panic).
- Add `TestIndex_MissingIndexDirectory`: delete an index directory entirely; verify `OpenForRead()` returns a clear error.

*Error paths:*
- Add `TestSearchTool_SearchAfterClose`: close the service, then attempt a search; verify "not ready" error (not panic).
- Add `TestReadTool_ReadAfterClose`: close the service, then attempt a file read; verify error.

*Edge cases:*
- Use `t.Parallel()` on independent tests for speed.
- All concurrent tests must pass with `-race` flag.

**Effort:** Medium

---

### CR-20: Use `fmt.Sprintf` consistently for error messages

**Files:** `internal/config/settings.go:208,234`

**Problem:** Some error messages use string concatenation instead of `fmt.Errorf`.

**Fix:** Replace `"transport must be 'stdio' or 'sse', got: " + s.Transport` with `fmt.Errorf(...)`.

**Test plan:**

*Regression:*
- Existing `TestValidateSettings_InvalidTransport` and `TestValidateSettings_UnknownAuthType` in `settings_test.go` must continue to pass. If error message format changes (e.g., quoting of values), update assertions to match.

*Validation:*
- Grep the codebase for all `errors.New(... +` and `fmt.Errorf(... +` patterns; convert every instance in a single pass.
- Verify each converted error message includes the dynamic value using `%s` or `%q` (quoted for user-facing values).

*Edge cases:*
- Add `TestValidateSettings_InvalidTransport_ErrorContainsValue`: verify the error message includes the invalid transport value (e.g., `got "foobar"`), not just the static text.
- Add `TestValidateSettings_UnknownAuthType_ErrorContainsValue`: verify the error message includes the unknown auth type value.

*No new test file changes required beyond assertion updates — this is a refactor covered by existing validation tests.*

**Effort:** Small

---

### CR-21: Document HTTPS deployment requirement

**File:** `README.md` or `docs/`

**Problem:** Auth middleware does not enforce TLS. Basic auth credentials are sent in plaintext HTTP headers without a reverse proxy.

**Fix:** Add a security section to docs with deployment recommendations (TLS termination, rate limiting, secrets management).

**Test plan:**
- No code tests — this is a documentation-only change.
- Review the new security section for accuracy against the actual auth middleware behavior in `internal/auth/middleware.go`.
- Verify `docker-compose.yml` example is consistent with the documented recommendations.
- Verify `README.md` renders correctly (no broken markdown).

**Effort:** Small

---

## Summary

| Severity | Count | IDs |
|----------|------:|-----|
| Critical | 2 | CR-1, CR-2 |
| High | 3 | CR-3, CR-4, CR-5 |
| Medium | 8 | CR-6 – CR-13 |
| Low | 8 | CR-14 – CR-21 |
| **Total** | **21** | |
