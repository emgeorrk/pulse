---
name: errors
description: Review changed and surrounding code for error handling violations and fix them. Use when checking or fixing error conventions in the project: no dynamic errors, sentinel errors only.
user-invocable: false
---

Review all changed and surrounding code in the current diff for error handling violations, then fix each one.

## Rules

### 1. Never create dynamic errors

Do not use `fmt.Errorf(...)` or `errors.New(...)` to construct ad-hoc error values at the call site. Every error must be a package-level sentinel.

**Wrong:**

```go
return fmt.Errorf("user %s not found", id)
return errors.New("connection failed")
```

### 2. Use sentinel errors

Each package declares its sentinels in a dedicated `errors.go` file
(see `internal/sensors/errors.go`). Keep them unexported (`errFoo`) unless
callers in other packages need `errors.Is` on them:

```go
package sensors

import "errors"

var (
    errCPUInfo = errors.New("host_processor_info failed")
    errSMCRead = errors.New("SMC read failed")
)
```

Wrap sentinels to add context using `fmt.Errorf` with `%w`:

```go
return fmt.Errorf("%w: kern_return_t %d", errCPUInfo, int(kr))
```

Wrapping an underlying error you received (syscall, stdlib) with `%w` is also
fine: `fmt.Errorf("sysctl hw.memsize: %w", err)`.

Do not create anonymous errors — always wrap a sentinel or a received error.

Check errors with `errors.Is`:

```go
if errors.Is(err, errSMCRead) {
    // handle SMC read failure
}
```

### 3. Test files

Test files (`*_test.go`) are exempt from these rules. They may use `fmt.Errorf` or `errors.New` freely to simulate error conditions in mocks and assertions.

## Checklist

For every piece of code you write or modify:

- [ ] No `fmt.Errorf(...)` or `errors.New(...)` producing anonymous errors in production code
- [ ] All packages declare their sentinels (`var errFoo = errors.New(...)`) in the package's `errors.go`
- [ ] Errors are wrapped with `fmt.Errorf("...: %w", errSentinel)` when adding context
- [ ] New error scenarios have a corresponding sentinel `var`
