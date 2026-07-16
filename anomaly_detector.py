import sys
import json
import os
from http.server import HTTPServer, BaseHTTPRequestHandler

# Import transformers. We will import on startup to fail early if missing.
try:
    from transformers import pipeline
except ImportError:
    print("[AI Error] Hugging Face 'transformers' package is not installed. Please run: pip install transformers torch")
    sys.exit(1)

print("[AI] Loading zero-shot classification model (typeform/distilbert-base-uncased-mnli)...")
try:
    classifier = pipeline(
        "zero-shot-classification", 
        model="typeform/distilbert-base-uncased-mnli", 
        device=-1 # run on CPU
    )
    print("[AI] Model loaded successfully!")
except Exception as e:
    print(f"[AI Error] Failed to load model: {e}")
    sys.exit(1)

CANDIDATE_LABELS = [
    "database error",
    "authentication failure",
    "network timeout",
    "normal client request"
]

PLAYBOOKS = {
    "database error": "Root Cause: Database connection pool exhaustion, database lock contention, or slow query execution. Playbook: Check DB connection pool limits, verify active DB hosts, inspect slow query logs, and check CPU/memory usage on the DB server.",
    "authentication failure": "Root Cause: Unauthorized request, expired OAuth token, or authentication provider outage. Playbook: Audit token expiration timestamps, inspect login route logs, verify CORS policies, and confirm status of Identity Provider.",
    "network timeout": "Root Cause: Network routing issue, downstream API gateway timeout, or healthy hosts depletion. Playbook: Verify Load Balancer target group health check states, inspect external downstream service metrics, and audit connection timeouts.",
    "normal client request": "Healthy system state. No action needed."
}

class DiagnosticHandler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        # Silence default stdout logs to avoid cluttering Go console output
        pass

    def do_POST(self):
        if self.path == '/analyze':
            content_length = int(self.headers.get('Content-Length', 0))
            post_data = self.rfile.read(content_length)
            
            try:
                req = json.loads(post_data.decode('utf-8'))
                log_line = req.get('log_line', '')
                
                if not log_line:
                    self.send_error_response(400, "Bad Request: 'log_line' field is required")
                    return
                
                # Perform zero-shot classification
                result = classifier(log_line, CANDIDATE_LABELS)
                
                predicted_label = result['labels'][0]
                confidence_score = result['scores'][0]
                playbook = PLAYBOOKS.get(predicted_label, "No playbook available.")
                
                response_data = {
                    "diagnosis": predicted_label.upper(),
                    "playbook": playbook,
                    "score": confidence_score
                }
                
                self.send_json_response(200, response_data)
                
            except Exception as e:
                self.send_error_response(500, f"Inference Error: {str(e)}")
        else:
            self.send_error_response(404, "Not Found")

    def send_json_response(self, status, data):
        self.send_response(status)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Access-Control-Allow-Origin', '*')
        self.end_headers()
        self.wfile.write(json.dumps(data).encode('utf-8'))

    def send_error_response(self, status, message):
        self.send_json_response(status, {"error": message})

def run(port=8086):
    server_address = ('127.0.0.1', port)
    httpd = HTTPServer(server_address, DiagnosticHandler)
    print(f"[AI] Telemetry Server listening on http://127.0.0.1:{port}")
    try:
        httpd.serve_forever()
    except KeyboardInterrupt:
        pass
    print("[AI] Telemetry Server stopped.")

if __name__ == '__main__':
    port = 8086
    if len(sys.argv) > 1:
        try:
            port = int(sys.argv[1])
        except ValueError:
            pass
    run(port)
