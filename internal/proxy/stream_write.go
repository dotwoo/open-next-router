package proxy

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/pkg/trafficdump"
)

type countingWriter struct {
	n int64
	w io.Writer
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.n += int64(n)
	return n, err
}

func streamToDownstream(
	gc *gin.Context,
	meta *dslmeta.Meta,
	respDir dslconfig.ResponseDirective,
	resp *http.Response,
	usageTail *tailBuffer,
	dump *streamDumpState,
) (int64, error) {
	needSSEOps := len(respDir.JSONOps) > 0 || len(respDir.SSEJSONDelIf) > 0
	useStrategyTransform := strings.TrimSpace(respDir.Op) == "sse_parse" && strings.EqualFold(strings.TrimSpace(respDir.Mode), "openai_responses_to_openai_chat_chunks")

	var src io.Reader = resp.Body
	var upstreamDump *limitedBuffer
	var proxyDump *limitedBuffer

	rec := trafficdump.FromContext(gc)
	if rec != nil && rec.MaxBytes() > 0 {
		upstreamDump = &limitedBuffer{limit: rec.MaxBytes()}
		proxyDump = &limitedBuffer{limit: rec.MaxBytes()}
	}

	if useStrategyTransform {
		pr, pw := io.Pipe()

		// Build upstream source for the strategy transform (decode gzip when needed).
		var upSrc io.Reader = resp.Body
		var closeUp func() error
		ce := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
		if ce == contentEncodingGzip {
			gr, err := gzip.NewReader(resp.Body)
			if err != nil {
				return 0, err
			}
			upSrc = gr
			closeUp = gr.Close
			// Ensure downstream doesn't see upstream encoding.
			gc.Writer.Header().Del("Content-Encoding")
		} else if ce != "" && ce != contentEncodingIdentity {
			return 0, fmt.Errorf("cannot transform encoded upstream response (Content-Encoding=%q)", resp.Header.Get("Content-Encoding"))
		}

		if upstreamDump != nil {
			upSrc = io.TeeReader(upSrc, upstreamDump)
		}

		// Override to downstream chat SSE.
		gc.Writer.Header().Set("Content-Type", "text/event-stream")
		gc.Writer.Header().Set("Cache-Control", "no-cache")
		gc.Status(resp.StatusCode)

		go func() {
			if closeUp != nil {
				defer func() { _ = closeUp() }()
			}
			err := dslconfig.TransformOpenAIResponsesSSEToChatCompletionsSSE(upSrc, pw)
			_ = pw.CloseWithError(err)
		}()

		src = pr
	} else {
		// passthrough
		gc.Status(resp.StatusCode)

		// If we need to parse/modify SSE, we must operate on decoded text.
		ce := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
		if needSSEOps {
			if ce == contentEncodingGzip {
				gr, err := gzip.NewReader(resp.Body)
				if err != nil {
					return 0, err
				}
				defer func() { _ = gr.Close() }()
				src = gr
				gc.Writer.Header().Del("Content-Encoding")
			} else if ce != "" && ce != contentEncodingIdentity {
				return 0, fmt.Errorf("cannot transform encoded upstream response (Content-Encoding=%q)", resp.Header.Get("Content-Encoding"))
			}
		}
	}

	if upstreamDump != nil && !useStrategyTransform {
		src = io.TeeReader(src, upstreamDump)
	}

	// Always tee the post-strategy stream into usageTail (pre-response-ops).
	src = io.TeeReader(src, usageTail)

	dst := io.Writer(gc.Writer)
	if proxyDump != nil {
		dst = io.MultiWriter(dst, proxyDump)
	}
	cw := &countingWriter{w: dst}

	ctLower := strings.ToLower(strings.TrimSpace(gc.Writer.Header().Get("Content-Type")))
	if ctLower == "" {
		ctLower = strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	}
	isSSE := strings.Contains(ctLower, "text/event-stream")

	var err error
	if needSSEOps && isSSE {
		err = dslconfig.TransformSSEEventDataJSON(src, cw, meta, respDir.SSEJSONDelIf, respDir.JSONOps)
	} else {
		_, err = io.Copy(cw, src)
	}

	if dump != nil && upstreamDump != nil && proxyDump != nil {
		dump.SetUpstream(upstreamDump.Bytes(), upstreamDump.Truncated())
		dump.SetProxy(proxyDump.Bytes(), proxyDump.Truncated())
	}

	return cw.n, err
}

// streamTransformedOpenAIResponses and streamPassthrough were merged into streamToDownstream
// to support response-phase SSE JSON mutations (json_* / sse_json_del_if).
