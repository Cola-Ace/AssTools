package output

import (
	"io"
)

// Writer records the first write failure while retaining the underlying writer
// for callers that need to inspect it (for example, terminal color detection).
type Writer struct {
	writer io.Writer
	err    error
}

func Track(writer io.Writer) *Writer {
	if tracked, ok := writer.(*Writer); ok {
		return tracked
	}
	return &Writer{writer: writer}
}

func (writer *Writer) Write(data []byte) (int, error) {
	if writer.err != nil {
		return 0, writer.err
	}
	n, err := writer.writer.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil {
		writer.err = err
	}
	return n, err
}

func (writer *Writer) Err() error {
	return writer.err
}

func (writer *Writer) Unwrap() io.Writer {
	return writer.writer
}
