package server

import (
	"bytes"
	"context"
	json "github.com/goccy/go-json"
	"io"
	"net/http"
	"strings"

	"aurora/internal/core"
)

const requestSelectorPeekLimit int64 = 64 * 1024

type peekedBodyKeyType struct{}
var peekedBodyKey = peekedBodyKeyType{}

func GetPeekedBody(ctx context.Context) []byte {
	if ctx == nil {
		return nil
	}
	if v := ctx.Value(peekedBodyKey); v != nil {
		return v.([]byte)
	}
	return nil
}

type requestBodySelectorHints struct {
	model    string
	provider string
	stream   bool
	parsed   bool
	complete bool
}

func seedRequestBodySelectorHints(req *http.Request, bodyMode core.BodyMode, env *core.WhiteBoxPrompt) {
	if !shouldPeekRequestBodySelectors(req, bodyMode, env) {
		return
	}

	hints := peekRequestBodySelectorHints(req, requestSelectorPeekLimit)
	if !hints.parsed || !hints.complete {
		return
	}
	core.ApplyBodySelectorHints(env, hints.model, hints.provider, hints.stream)
}

func shouldPeekRequestBodySelectors(req *http.Request, bodyMode core.BodyMode, env *core.WhiteBoxPrompt) bool {
	if req == nil || req.Body == nil || env == nil {
		return false
	}
	switch bodyMode {
	case core.BodyModeJSON:
		return true
	case core.BodyModeOpaque:
		return contentTypeLooksJSON(req.Header.Get("Content-Type"))
	default:
		return false
	}
}

func contentTypeLooksJSON(contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	return strings.Contains(contentType, "json")
}

func peekRequestBodySelectorHints(req *http.Request, limit int64) requestBodySelectorHints {
	if req == nil || req.Body == nil || limit <= 0 {
		return requestBodySelectorHints{}
	}

	originalBody := req.Body

	if minimalBenchModeEnabled() {
		var buf [1024]byte
		n, err := originalBody.Read(buf[:])
		if err != nil && err != io.EOF {
			req.Body = originalBody
			return requestBodySelectorHints{}
		}
		bodyBytes := make([]byte, n)
		copy(bodyBytes, buf[:n])

		isStream := bytes.Contains(bodyBytes, []byte(`"stream":true`)) || bytes.Contains(bodyBytes, []byte(`"stream": true`))

		model := ""
		if idx := bytes.Index(bodyBytes, []byte(`"model"`)); idx != -1 {
			rest := bodyBytes[idx:]
			if valIdx := bytes.IndexByte(rest, ':'); valIdx != -1 {
				val := rest[valIdx+1:]
				val = bytes.TrimSpace(val)
				if len(val) > 0 && val[0] == '"' {
					val = val[1:]
					if endIdx := bytes.IndexByte(val, '"'); endIdx != -1 {
						model = string(val[:endIdx])
					}
				}
			}
		}

		provider := ""
		if idx := bytes.Index(bodyBytes, []byte(`"provider"`)); idx != -1 {
			rest := bodyBytes[idx:]
			if valIdx := bytes.IndexByte(rest, ':'); valIdx != -1 {
				val := rest[valIdx+1:]
				val = bytes.TrimSpace(val)
				if len(val) > 0 && val[0] == '"' {
					val = val[1:]
					if endIdx := bytes.IndexByte(val, '"'); endIdx != -1 {
						provider = string(val[:endIdx])
					}
				}
			}
		}

		req.Body = &combinedReadCloser{
			Reader: io.MultiReader(bytes.NewReader(bodyBytes), originalBody),
			rc:     originalBody,
		}

		*req = *req.WithContext(context.WithValue(req.Context(), peekedBodyKey, bodyBytes))

		parsed := false
		complete := false
		if model != "" && provider != "" {
			parsed = true
		} else if model != "" {
			parsed = false
			complete = false
		} else if provider != "" {
			parsed = false
			complete = false
		} else {
			parsed = true
			complete = true
		}

		return requestBodySelectorHints{
			model:    model,
			provider: provider,
			stream:   isStream,
			parsed:   parsed,
			complete: complete,
		}
	}

	var consumed bytes.Buffer
	limited := io.LimitReader(originalBody, limit)
	hints := decodeRequestBodySelectorHints(io.TeeReader(limited, &consumed))

	req.Body = &combinedReadCloser{
		Reader: io.MultiReader(bytes.NewReader(consumed.Bytes()), originalBody),
		rc:     originalBody,
	}
	return hints
}

func decodeRequestBodySelectorHints(r io.Reader) requestBodySelectorHints {
	dec := json.NewDecoder(r)
	token, err := dec.Token()
	if err != nil {
		return requestBodySelectorHints{}
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return requestBodySelectorHints{}
	}

	var hints requestBodySelectorHints
	for dec.More() {
		keyToken, err := dec.Token()
		if err != nil {
			return requestBodySelectorHints{}
		}
		key, ok := keyToken.(string)
		if !ok {
			return requestBodySelectorHints{}
		}

		switch key {
		case "model":
			model, ok, err := readOptionalJSONString(dec)
			if err != nil || !ok {
				return requestBodySelectorHints{}
			}
			hints.model = model
			if model != "" && hints.provider != "" {
				hints.parsed = true
				return hints
			}
			if model != "" {
				return hints
			}
		case "provider":
			provider, ok, err := readOptionalJSONString(dec)
			if err != nil || !ok {
				return requestBodySelectorHints{}
			}
			hints.provider = provider
			if hints.provider != "" && hints.model != "" {
				hints.parsed = true
				return hints
			}
		case "stream":
			stream, ok, err := readOptionalJSONBool(dec)
			if err != nil || !ok {
				return requestBodySelectorHints{}
			}
			hints.stream = stream
		default:
			if err := skipJSONValue(dec); err != nil {
				return requestBodySelectorHints{}
			}
		}
	}

	hints.parsed = true
	hints.complete = true
	return hints
}

func readOptionalJSONString(dec *json.Decoder) (string, bool, error) {
	token, err := dec.Token()
	if err != nil {
		return "", false, err
	}
	switch value := token.(type) {
	case string:
		return value, true, nil
	case nil:
		return "", true, nil
	default:
		return "", false, nil
	}
}

func readOptionalJSONBool(dec *json.Decoder) (bool, bool, error) {
	token, err := dec.Token()
	if err != nil {
		return false, false, err
	}
	switch value := token.(type) {
	case bool:
		return value, true, nil
	case nil:
		return false, true, nil
	default:
		return false, false, nil
	}
}

func skipJSONValue(dec *json.Decoder) error {
	token, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}

	switch delim {
	case '{', '[':
		depth := 1
		for depth > 0 {
			token, err = dec.Token()
			if err != nil {
				return err
			}
			nested, ok := token.(json.Delim)
			if !ok {
				continue
			}
			switch nested {
			case '{', '[':
				depth++
			case '}', ']':
				depth--
			}
		}
	}
	return nil
}
