package lang

import (
	"bytes"
	"io"
)

type InputFile interface {
	Name() string
	NewReader() io.Reader
}

type StreamingInputFile struct {
	name   string
	reader io.Reader
}

func NewStreamingInputFile(name string, reader io.Reader) InputFile {
	return &StreamingInputFile{name, reader}
}
func (f *StreamingInputFile) Name() string { return f.name }
func (f *StreamingInputFile) NewReader() io.Reader {
	return f.reader
}

type BufferedInputFile struct {
	name    string
	content []byte
}

func NewBufferedInputFile(name string, content []byte) InputFile {
	return &BufferedInputFile{name, content}
}
func (f *BufferedInputFile) Name() string { return f.name }
func (f *BufferedInputFile) NewReader() io.Reader {
	return bytes.NewReader(f.content)
}
