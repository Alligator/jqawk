package lang

type SyntaxError struct {
	Message string
	Line    int
	Col     int
	SrcLine string
}

func (err SyntaxError) Error() string {
	return err.Message
}

type RuntimeError struct {
	Message string
	Line    int
	Col     int
	SrcLine string
}

func (err RuntimeError) Error() string {
	return err.Message
}

type JsonError struct {
	Message  string
	FileName string
}

func (err JsonError) Error() string {
	return err.Message
}
