package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/itchyny/gojq"
	"k8s.io/client-go/util/jsonpath"
)

func InSlice[T comparable](slice []T, el T) bool {
	for _, v := range slice {
		if el == v {
			return true
		}
	}
	return false
}

func DeleteMultiple[K comparable, V any](m map[K]V, removeKeys []K) {
	for _, k := range removeKeys {
		delete(m, k)
	}
}

func CopyResponse(resp *response, upstream *http.Response, customBody []byte, filterHeaders []string) error {
	for k, v := range upstream.Header {
		if filterHeaders == nil || !InSlice(filterHeaders, k) {
			for _, vv := range v {
				resp.AddHeader(k, vv)
			}
		}
	}

	resp.statusCode = upstream.StatusCode

	if customBody != nil {
		resp.body = customBody
	} else {
		var writer = bytes.NewBuffer(resp.body)
		_, err := io.Copy(writer, upstream.Body)
		if err != nil {
			return err
		}
		resp.body = writer.Bytes()

	}
	return nil
}

func ParseJsonPath(inputJson []byte, inputJsonPath string) ([]byte, error) {
	j := jsonpath.New("jsonpath-parser")

	err := j.Parse(inputJsonPath)
	if err != nil {
		return nil, err
	}

	var jsonData interface{}
	err = json.Unmarshal(inputJson, &jsonData)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	err = j.Execute(&buf, jsonData)

	return buf.Bytes(), err
}

func ParseJQ(inputJson []byte, inputJQ string) (string, error) {
	query, err := gojq.Parse(inputJQ)
	if err != nil {
		return "", err
	}

	var jsonData interface{}
	err = json.Unmarshal(inputJson, &jsonData)
	if err != nil {
		return "", err
	}

	iter := query.Run(jsonData)
	var result []string
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			return "", err
		}

		if b, err := json.Marshal(v); err == nil {
			result = append(result, string(b))
		} else {
			return "", err
		}
	}

	return strings.Join(result[:], "\n"), nil
}
