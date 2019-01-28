package utils

func Clone(src []byte) []byte {
	clone := make([]byte, len(src))
	copy(clone, src)

	return clone
}
