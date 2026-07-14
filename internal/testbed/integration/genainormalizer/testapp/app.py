import json
import os
import threading
import time
from http.server import BaseHTTPRequestHandler, HTTPServer

from openai import OpenAI
from openinference.instrumentation.openai import OpenAIInstrumentor
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk import trace as trace_sdk
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.semconv.resource import ResourceAttributes

MOCK_PORT = 8080
CALL_INTERVAL_SECONDS = 2

# Two representative OpenInference model shapes:
#   - OpenAI:       gpt-4o
#   - AWS Bedrock:  anthropic.claude-3-sonnet-20240229-v1:0
MODELS = [
    os.environ.get("MODEL_OPENAI", "gpt-4o"),
    os.environ.get("MODEL_BEDROCK", "anthropic.claude-3-sonnet-20240229-v1:0"),
]


class MockOpenAIHandler(BaseHTTPRequestHandler):
    """Minimal OpenAI-compatible HTTP server returning canned responses."""

    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        body = json.loads(self.rfile.read(length)) if length else {}
        model = body.get("model", "gpt-4o")
        payload = json.dumps({
            "id": "chatcmpl-test",
            "object": "chat.completion",
            "created": int(time.time()),
            "model": model,
            "choices": [
                {
                    "index": 0,
                    "message": {"role": "assistant", "content": "Hello!"},
                    "finish_reason": "stop",
                }
            ],
            "usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
        }).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def log_message(self, *_args):
        pass  # suppress access logs


def start_mock_server() -> None:
    server = HTTPServer(("0.0.0.0", MOCK_PORT), MockOpenAIHandler)
    threading.Thread(target=server.serve_forever, daemon=True).start()


def setup_tracing() -> None:
    service_name = os.environ.get("OTEL_SERVICE_NAME", "test-genainormalizer-openinference")
    resource = Resource.create({ResourceAttributes.SERVICE_NAME: service_name})
    provider = trace_sdk.TracerProvider(resource=resource)
    # OTEL_EXPORTER_OTLP_ENDPOINT is read automatically by OTLPSpanExporter
    provider.add_span_processor(BatchSpanProcessor(OTLPSpanExporter()))
    OpenAIInstrumentor().instrument(tracer_provider=provider)


def main() -> None:
    start_mock_server()
    time.sleep(0.5)

    setup_tracing()

    client = OpenAI(
        base_url=f"http://localhost:{MOCK_PORT}/v1",
        api_key="test-key",
    )

    while True:
        for model in MODELS:
            try:
                client.chat.completions.create(
                    model=model,
                    messages=[{"role": "user", "content": "Write a haiku."}],
                )
            except Exception as exc:
                print(f"error calling model {model}: {exc}", flush=True)
        time.sleep(CALL_INTERVAL_SECONDS)


if __name__ == "__main__":
    main()
