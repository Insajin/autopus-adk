package content

func rewriteCanonicalSkillReferences(body string, resolve func(name string) string) string {
	if resolve == nil {
		return body
	}

	return canonicalSkillRefRe.ReplaceAllStringFunc(body, func(match string) string {
		sub := canonicalSkillRefRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		if resolved := resolve(sub[1]); resolved != "" {
			return resolved
		}
		return match
	})
}
