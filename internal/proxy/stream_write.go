package proxy

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/pkg/trafficdump"
)

func streamToDownstream(
	gc *gin.Context,
	respDir dslconfig.ResponseDirective,
	resp *http.Response,
	usageTail *tailBuffer,
	dump *streamDumpState,
) error {
	if strings.TrimSpace(respDir.Op) == "sse_parse" && strings.EqualFold(strings.TrimSpace(respDir.Mode), "openai_responses_to_openai_chat_chunks") {
		return streamTransformedOpenAIResponses(gc, resp, usageTail, dump)
	}
	return streamPassthrough(gc, resp, usageTail, dump)
}

func streamTransformedOpenAIResponses(gc *gin.Context, resp *http.Response, usageTail *tailBuffer, dump *streamDumpState) error {
	var src io.Reader = resp.Body
	ce := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	if ce == "gzip" {
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return err
		}
		defer func() { _ = gr.Close() }()
		src = gr
		// Override encoding for downstream.
		gc.Writer.Header().Del("Content-Encoding")
	} else if ce != "" && ce != contentEncodingIdentity {
		return fmt.Errorf("cannot transform encoded upstream response (Content-Encoding=%q)", resp.Header.Get("Content-Encoding"))
	}

	// Override to downstream chat SSE.
	gc.Writer.Header().Set("Content-Type", "text/event-stream")
	gc.Writer.Header().Set("Cache-Control", "no-cache")
	gc.Status(resp.StatusCode)

	rec := trafficdump.FromContext(gc)
	if rec == nil || rec.MaxBytes() <= 0 {
		dst := io.MultiWriter(gc.Writer, usageTail)
		return dslconfig.TransformOpenAIResponsesSSEToChatCompletionsSSE(src, dst)
	}

	upDump := &limitedBuffer{limit: rec.MaxBytes()}
	prDump := &limitedBuffer{limit: rec.MaxBytes()}
	src = io.TeeReader(src, upDump)
	dst := io.MultiWriter(gc.Writer, prDump, usageTail)
	err := dslconfig.TransformOpenAIResponsesSSEToChatCompletionsSSE(src, dst)
	if dump != nil {
		dump.SetUpstream(upDump.Bytes(), upDump.Truncated())
		dump.SetProxy(prDump.Bytes(), prDump.Truncated())
	}
	return err
}

func streamPassthrough(gc *gin.Context, resp *http.Response, usageTail *tailBuffer, dump *streamDumpState) error {
	gc.Status(resp.StatusCode)

	rec := trafficdump.FromContext(gc)
	if rec == nil || rec.MaxBytes() <= 0 {
		tee := io.TeeReader(resp.Body, usageTail)
		_, err := io.Copy(gc.Writer, tee)
		return err
	}

	buf := &limitedBuffer{limit: rec.MaxBytes()}
	tee := io.TeeReader(resp.Body, io.MultiWriter(buf, usageTail))
	_, err := io.Copy(gc.Writer, tee)
	if dump != nil {
		dump.SetUpstream(buf.Bytes(), buf.Truncated())
		dump.SetProxy(buf.Bytes(), buf.Truncated())
	}
	return err
}
