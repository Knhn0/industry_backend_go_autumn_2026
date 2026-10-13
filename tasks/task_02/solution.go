package main

func rotateRunes(s string, shift int) string {
	runeArr := []rune(s)
	n := len(runeArr)
	if n == 0 {
		return s
	}

	eff := shift % n
	if eff < 0 {
		eff += n
	}

	if eff == 0 {
		return string(runeArr)
	}

	out := make([]rune, n)
	copy(out, runeArr[eff:])
	copy(out[n-eff:], runeArr[:eff])
	return string(out)
}
