package dag

type diagnosticBase struct {
	severity dag.Severity
	summary  string
	detail   string
}

func Diagnostic(severity dag.Severity, summary, detail string) dag.Diagnostic {
	return diagnosticBase{
		severity: severity,
		summary:  summary,
		detail:   detail,
	}
}

func (d diagnosticBase) Severity() dag.Severity {
	return d.severity
}

func (d diagnosticBase) Description() dag.Description {
	return dag.Description{
		Summary: d.summary,
		Detail:  d.detail,
	}
}