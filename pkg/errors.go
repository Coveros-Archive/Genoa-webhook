package pkg

type ErrorFileContentIsNotReleaseFile struct {
	Message string
}

func (e ErrorFileContentIsNotReleaseFile) Error() string {
	return e.Message
}
