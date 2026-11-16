package common

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/jinzhu/copier"
)

// Copy JSON buffers in bulk. copier otherwise reflects over each byte, causing
// millions of allocations for requests containing inline images. Keep separate
// buffers so channel conversion and retries cannot mutate the original request.
var deepCopyByteConverters = []copier.TypeConverter{
	{
		SrcType: json.RawMessage(nil), DstType: json.RawMessage(nil),
		Fn: func(src interface{}) (interface{}, error) {
			return json.RawMessage(bytes.Clone(src.(json.RawMessage))), nil
		},
	},
	{
		SrcType: []byte(nil), DstType: []byte(nil),
		Fn: func(src interface{}) (interface{}, error) {
			return bytes.Clone(src.([]byte)), nil
		},
	},
}

func DeepCopy[T any](src *T) (*T, error) {
	if src == nil {
		return nil, fmt.Errorf("copy source cannot be nil")
	}
	var dst T
	err := copier.CopyWithOption(&dst, src, copier.Option{DeepCopy: true, IgnoreEmpty: true, Converters: deepCopyByteConverters})
	if err != nil {
		return nil, err
	}
	return &dst, nil
}
