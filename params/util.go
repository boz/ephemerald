package params

func MergeDefaultsWithOverride(p Params, override map[string]string, defaults map[string]string) Params {
	return p.MergeVars(override).MergeVars(defaults)
}
