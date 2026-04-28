package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/itchyny/gojq"
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

func ParseJQ(ctx context.Context, inputJson []byte, inputJQ string, maxResults int) (string, error) {
	query, err := gojq.Parse(inputJQ)
	if err != nil {
		return "", fmt.Errorf("invalid jq expression")
	}

	var jsonData interface{}
	err = json.Unmarshal(inputJson, &jsonData)
	if err != nil {
		return "", fmt.Errorf("invalid JSON input")
	}

	iter := query.RunWithContext(ctx, jsonData)
	var result []string
	for i := 0; i < maxResults; i++ {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			return "", fmt.Errorf("jq execution failed: %w", err)
		}

		if b, err := json.Marshal(v); err == nil {
			result = append(result, string(b))
		} else {
			return "", fmt.Errorf("failed to marshal jq result")
		}
	}

	return strings.Join(result[:], "\n"), nil
}

// parseAuthorizationHeaderWithDoubleTokens parses an authorization header that may contain two tokens separated by a comma.
// It returns the first token and the second token (if present). If the second token is absent, it returns an empty string for it.
// If the header is empty or contains more than two tokens, it returns an error.
func parseAuthorizationHeaderWithDoubleTokens(authHeader string) (string, string, error) {
	if authHeader == "" {
		return "", "", fmt.Errorf("authorization header is empty")
	}

	tokens := strings.Split(authHeader, ",")
	if len(tokens) > 2 {
		return "", "", fmt.Errorf("authorization header must contain two or less tokens separated by a space")
	}
	if len(tokens) == 1 {
		return tokens[0], "", nil
	}
	return tokens[0], tokens[1], nil
}
