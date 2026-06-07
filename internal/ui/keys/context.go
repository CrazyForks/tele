package keys

import "sort"

// Resolve returns the Action for the given key in ctx, falling back to
// ContextGlobal. If the raw key matches nothing, it is retried after translating
// a non-Latin keyboard layout to its Latin equivalent (same physical key).
func (km KeyMap) Resolve(ctx Context, key string) Action {
	if a := km.resolveExact(ctx, key); a != ActionNone {
		return a
	}
	if nk := normalizeKey(key); nk != key {
		return km.resolveExact(ctx, nk)
	}
	return ActionNone
}

func (km KeyMap) resolveExact(ctx Context, key string) Action {
	if ctxMap, ok := km[ctx]; ok {
		if action, ok := ctxMap[key]; ok {
			return action
		}
	}
	if global, ok := km[ContextGlobal]; ok {
		if action, ok := global[key]; ok {
			return action
		}
	}
	return ActionNone
}

// KeyFor returns the preferred key bound to action in ctx.
// When multiple keys map to the same action, the shortest is chosen;
// ties are broken alphabetically. Returns "" if no binding exists.
// Does not fall back to ContextGlobal.
func (km KeyMap) KeyFor(ctx Context, action Action) string {
	ctxMap, ok := km[ctx]
	if !ok {
		return ""
	}
	var candidates []string
	for key, a := range ctxMap {
		if a == action {
			candidates = append(candidates, key)
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.Slice(candidates, func(i, j int) bool {
		li, lj := len(candidates[i]), len(candidates[j])
		if li != lj {
			return li < lj
		}
		return candidates[i] < candidates[j]
	})
	return candidates[0]
}
