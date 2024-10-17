package pagination

func Slice[T any](slice []T, p *Pagination) []T {
	if len(slice) < int(p.Offset()) {
		return make([]T, 0)
	}

	if len(slice) < int(p.Offset()+p.Limit()) {
		return slice[p.Offset():]
	}

	return slice[p.Offset() : p.Offset()+p.Limit()]
}
