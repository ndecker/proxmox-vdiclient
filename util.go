package main

func filter[T any](in []T, f func(r T) bool) []T {
	res := make([]T, 0, len(in))
	for _, v := range in {
		if f(v) {
			res = append(res, v)
		}
	}
	return res
}
