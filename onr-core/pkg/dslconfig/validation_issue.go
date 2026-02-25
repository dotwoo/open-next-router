package dslconfig

// ValidationIssue carries structured semantic validation context.
// It wraps the original validation error message and adds directive/scope hints
// for tooling (for example, LSP diagnostics positioning).
type ValidationIssue struct {
	Directive string
	Scope     string
	Err       error
}

func (e *ValidationIssue) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *ValidationIssue) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func validationIssue(err error, scope, directive string) error {
	if err == nil {
		return nil
	}
	return &ValidationIssue{
		Directive: directive,
		Scope:     scope,
		Err:       err,
	}
}
