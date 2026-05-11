package responses

import (
	"context"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func TestConvertOpenAIChatCompletionsResponseToResponsesNonStreamReasoningAlias(t *testing.T) {
	raw := []byte(`{"id":"chatcmpl-827","object":"chat.completion","created":1778504096,"model":"deepseek-v4-flash","choices":[{"index":0,"message":{"role":"assistant","content":"","reasoning":"non standard reasoning text"},"finish_reason":"stop"}],"usage":{"prompt_tokens":9,"completion_tokens":8,"total_tokens":17}}`)

	out := ConvertOpenAIChatCompletionsResponseToOpenAIResponsesNonStream(context.Background(), "deepseek-v4-flash", nil, []byte(`{"model":"deepseek-v4-flash"}`), raw, nil)
	root := gjson.Parse(out)
	if got := root.Get("output.0.type").String(); got != "reasoning" {
		t.Fatalf("output.0.type = %q, want reasoning; body=%s", got, out)
	}
	if got := root.Get("output.0.summary.0.text").String(); got != "non standard reasoning text" {
		t.Fatalf("reasoning summary = %q", got)
	}
	if got := root.Get("usage.input_tokens").Int(); got != 9 {
		t.Fatalf("usage.input_tokens = %d", got)
	}
}

func TestConvertOpenAIChatCompletionsResponseToResponsesStreamReasoningAliasAndResponsesUsage(t *testing.T) {
	var param any
	chunk := []byte(`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1778504096,"model":"deepseek-v4-flash","choices":[{"index":0,"delta":{"reasoning":"thinking"},"finish_reason":null}],"usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7,"input_tokens_details":{"cached_tokens":1},"output_tokens_details":{"reasoning_tokens":4}}}`)
	segments := ConvertOpenAIChatCompletionsResponseToOpenAIResponses(context.Background(), "deepseek-v4-flash", nil, []byte(`{"model":"deepseek-v4-flash"}`), chunk, &param)
	joined := strings.Join(segments, "\n")
	if !strings.Contains(joined, "response.reasoning_summary_text.delta") {
		t.Fatalf("expected reasoning delta event, got %s", joined)
	}

	finish := []byte(`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1778504096,"model":"deepseek-v4-flash","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`)
	segments = ConvertOpenAIChatCompletionsResponseToOpenAIResponses(context.Background(), "deepseek-v4-flash", nil, []byte(`{"model":"deepseek-v4-flash"}`), finish, &param)
	joined = strings.Join(segments, "\n")
	if !strings.Contains(joined, `"input_tokens":3`) || !strings.Contains(joined, `"output_tokens":4`) {
		t.Fatalf("expected responses-style usage to be preserved, got %s", joined)
	}
	if !strings.Contains(joined, `"cached_tokens":1`) || !strings.Contains(joined, `"reasoning_tokens":4`) {
		t.Fatalf("expected usage detail tokens to be preserved, got %s", joined)
	}
}
