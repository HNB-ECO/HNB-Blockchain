
package utils

func ErrToString(err error) string {
	if err != nil {
		return err.Error()
	}

	return "<clean>"
}
